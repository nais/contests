package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsHandlerExposesRuntimeAndProcessMetrics(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	newHandler(newRegistry(), testLogger()).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
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

func TestKafkaEndpointIsDisabledWithoutBrokers(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "")
	handler := newHandler(newRegistry(), testLogger())
	for _, path := range []string{"/ping", "/metrics", "/kafka", "/missing"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		want := http.StatusNotFound
		if path == "/ping" || path == "/metrics" {
			want = http.StatusOK
		}
		if recorder.Code != want {
			t.Errorf("%s status = %d, want %d", path, recorder.Code, want)
		}
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
