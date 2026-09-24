// Package dashboards guards the shipped Grafana dashboard against regressions.
package dashboards

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func load(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile("vsphere-hardware.json")
	if err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

// walk calls fn for every JSON object in v.
func walk(v any, fn func(map[string]any)) {
	switch x := v.(type) {
	case map[string]any:
		fn(x)
		for _, c := range x {
			walk(c, fn)
		}
	case []any:
		for _, c := range x {
			walk(c, fn)
		}
	}
}

// The dashboard must work both when imported and when file-provisioned, so every
// datasource reference has to use the "uid" key and the ${datasource} variable, not an
// import-only __inputs placeholder.
func TestDatasourceReferences(t *testing.T) {
	d := load(t)
	if _, ok := d["__inputs"]; ok {
		t.Error("__inputs is import-only and breaks file provisioning")
	}
	refs := 0
	walk(d, func(m map[string]any) {
		if _, bad := m["uuid"]; bad {
			t.Errorf(`datasource reference uses "uuid" (Grafana ignores it): %v`, m)
		}
		if m["type"] == "prometheus" {
			refs++
			if m["uid"] != "${datasource}" {
				t.Errorf(`datasource reference must be {"uid": "${datasource}"}, got %v`, m)
			}
		}
	})
	if refs == 0 {
		t.Error("no datasource references found")
	}
	if strings.Contains(mustMarshal(t, d), "DS_PROMETHEUS") {
		t.Error("import placeholder DS_PROMETHEUS left in the dashboard")
	}
}

func TestDatasourceVariable(t *testing.T) {
	vars := load(t)["templating"].(map[string]any)["list"].([]any)
	first := vars[0].(map[string]any)
	if first["type"] != "datasource" || first["name"] != "datasource" || first["query"] != "prometheus" {
		t.Errorf("first template variable must be the prometheus datasource selector, got %v", first)
	}
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Table panels join several instant queries with the "merge" transformation. Merge only
// combines rows whose common columns are identical, so per-query columns (__name__, Time)
// must be dropped first, or every host is split across one row per query.
func TestTablesStripPerQueryColumnsBeforeMerge(t *testing.T) {
	tables := 0
	walk(load(t), func(m map[string]any) {
		if m["type"] != "table" {
			return
		}
		tables++
		tr, _ := m["transformations"].([]any)
		if len(tr) < 2 {
			t.Errorf("table %v: expected organize+merge transformations", m["title"])
			return
		}
		first, _ := tr[0].(map[string]any)
		opts, _ := first["options"].(map[string]any)
		excl, _ := opts["excludeByName"].(map[string]any)
		if first["id"] != "organize" || excl["__name__"] != true || excl["Time"] != true {
			t.Errorf("table %v: first transformation must drop Time and __name__, got %v", m["title"], first)
		}
		second, _ := tr[1].(map[string]any)
		if second["id"] != "merge" {
			t.Errorf("table %v: second transformation must be merge, got %v", m["title"], second["id"])
		}
	})
	if tables == 0 {
		t.Error("no table panels found")
	}
}

// The grafana.com upload is derived from the main dashboard by Grafana's exporter. It must stay
// in sync (same panels and queries) and must not carry local datasource names.
func TestGrafanaComCopyMatchesMainDashboard(t *testing.T) {
	main := load(t)
	b, err := os.ReadFile("grafana-com/vsphere-hardware.json")
	if err != nil {
		t.Fatal(err)
	}
	var gc map[string]any
	if err := json.Unmarshal(b, &gc); err != nil {
		t.Fatal(err)
	}
	if inputs, _ := gc["__inputs"].([]any); len(inputs) != 1 || inputs[0].(map[string]any)["name"] != "DS_PROMETHEUS" {
		t.Errorf("grafana.com copy must declare exactly the DS_PROMETHEUS input, got %v", gc["__inputs"])
	}
	if gc["id"] != nil {
		t.Errorf("id must be null, got %v", gc["id"])
	}
	if gc["uid"] != main["uid"] || gc["title"] != main["title"] {
		t.Errorf("uid/title differ from main dashboard")
	}
	if got, want := signature(gc), signature(main); got != want {
		t.Errorf("panels/queries differ from the main dashboard; regenerate the grafana.com copy (see grafana-com/MAINTAINING.md)\n got: %s\nwant: %s", got, want)
	}
	for _, leak := range []string{"127.0.0.1", "prom-test-uid", "\"DS_PROM\"", "${DS_PROM}"} {
		if strings.Contains(string(b), leak) {
			t.Errorf("grafana.com copy contains %q", leak)
		}
	}
}

// signature reduces a dashboard to its panel titles/types and query expressions.
func signature(d map[string]any) string {
	var parts []string
	for _, p := range d["panels"].([]any) {
		pm := p.(map[string]any)
		parts = append(parts, pm["title"].(string)+"|"+pm["type"].(string))
		ts, _ := pm["targets"].([]any)
		for _, tg := range ts {
			parts = append(parts, tg.(map[string]any)["expr"].(string))
		}
	}
	for _, v := range d["templating"].(map[string]any)["list"].([]any) {
		parts = append(parts, "var:"+v.(map[string]any)["name"].(string))
	}
	return strings.Join(parts, "\n")
}
