package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

type fakeReady bool

func (f fakeReady) Ready() bool { return bool(f) }

func get(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code, rec.Body.String()
}

func TestEndpoints(t *testing.T) {
	reg := prometheus.NewRegistry()
	g := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_metric", Help: "h"})
	g.Set(1)
	reg.MustRegister(g)

	if code, body := get(t, newHandler(reg, fakeReady(true)), "/metrics"); code != 200 || !strings.Contains(body, "test_metric 1") {
		t.Errorf("/metrics = %d %q", code, body)
	}
	if code, _ := get(t, newHandler(reg, fakeReady(false)), "/healthz"); code != 200 {
		t.Errorf("/healthz must be 200 even when not ready, got %d", code)
	}
	if code, _ := get(t, newHandler(reg, fakeReady(false)), "/ready"); code != 503 {
		t.Errorf("/ready before first refresh = %d, want 503", code)
	}
	if code, _ := get(t, newHandler(reg, fakeReady(true)), "/ready"); code != 200 {
		t.Errorf("/ready after refresh = %d, want 200", code)
	}
	if code, _ := get(t, newHandler(reg, fakeReady(true)), "/nope"); code != 404 {
		t.Errorf("unknown path = %d, want 404", code)
	}
}
