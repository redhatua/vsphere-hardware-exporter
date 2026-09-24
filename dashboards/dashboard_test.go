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
