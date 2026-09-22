package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsHandlerExposesRuntimeAndProcessMetrics(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	metricsHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	for _, metric := range []string{
		"go_goroutines",
		"go_memstats_heap_alloc_bytes",
		"go_memstats_heap_inuse_bytes",
		"go_memstats_heap_sys_bytes",
		"go_memstats_heap_idle_bytes",
		"go_memstats_heap_released_bytes",
		"go_memstats_next_gc_bytes",
		"go_gc_duration_seconds",
		"process_resident_memory_bytes",
	} {
		if !strings.Contains(recorder.Body.String(), metric) {
			t.Errorf("metrics response does not contain %q", metric)
		}
	}
}
