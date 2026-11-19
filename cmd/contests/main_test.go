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
	newHandler(newRegistry()).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
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

func TestConfiguredChecks(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		config  string
		want    string
		wantErr bool
	}{
		{name: "defaults", want: "postgres,kafka,bucket,valkey"},
		{name: "subset", config: "postgres,bucket", want: "postgres,bucket"},
		{name: "network target only", config: "none"},
		{name: "none cannot be combined with checks", config: "none,postgres", wantErr: true},
		{name: "unknown", config: "postgres,missing", wantErr: true},
		{name: "duplicate", config: "bucket,bucket", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			checks, err := configuredChecks(tt.config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("configuredChecks() error = %v", err)
			}
			if tt.wantErr {
				return
			}
			var names []string
			for _, check := range checks {
				names = append(names, check.Name)
			}
			if got := strings.Join(names, ","); got != tt.want {
				t.Fatalf("checks = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOnlyHealthAndMetricsAreExposed(t *testing.T) {
	t.Parallel()
	handler := newHandler(newRegistry())
	for _, path := range []string{"/ping", "/metrics", "/postgres", "/kafka", "/bucket", "/valkey", "/statuscode/500"} {
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
