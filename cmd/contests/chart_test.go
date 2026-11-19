package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestChartRendersNetworkContractsAndConfiguredServices(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		extra       []string
		checks      string
		kinds       []string
		noPolicy    bool
		noAlerts    bool
		wantErr     bool
		extraTarget string
	}{
		{name: "defaults", checks: "postgres,kafka,bucket,valkey", kinds: []string{"Postgres", "Topic", "Valkey"}},
		{name: "postgres only", extra: []string{"contests.kafka.enabled=false", "contests.bucket.enabled=false", "contests.valkey.enabled=false"}, checks: "postgres", kinds: []string{"Postgres"}},
		{name: "no Kafka", extra: []string{"contests.kafka.enabled=false"}, checks: "postgres,bucket,valkey", kinds: []string{"Postgres", "Valkey"}},
		{name: "network only", extra: []string{"contests.postgres.enabled=false", "contests.kafka.enabled=false", "contests.bucket.enabled=false", "contests.valkey.enabled=false"}, checks: "none"},
		{name: "no denied probes", extra: []string{"contests.accessPolicy.enabled=false"}, checks: "postgres,kafka,bucket,valkey", kinds: []string{"Postgres", "Topic", "Valkey"}, noPolicy: true},
		{name: "no alerts", extra: []string{"alerts.enabled=false"}, checks: "postgres,kafka,bucket,valkey", kinds: []string{"Postgres", "Topic", "Valkey"}, noAlerts: true},
		{name: "additional target", extra: []string{"blackboxExporter.additionalTargetURLs[0]=https://example.com/health"}, checks: "postgres,kafka,bucket,valkey", kinds: []string{"Postgres", "Topic", "Valkey"}, extraTarget: "https://example.com/health"},
		{name: "missing Kafka pool", extra: []string{"contests.kafka.pool="}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"template", "contests", "../../charts/contests", "--namespace", "nais-system", "--set", "contests.kafka.pool=test"}
			for _, setting := range tt.extra {
				args = append(args, "--set", setting)
			}
			output, err := exec.CommandContext(t.Context(), "helm", args...).CombinedOutput()
			if (err != nil) != tt.wantErr {
				t.Fatalf("helm template: %v\n%s", err, output)
			}
			if tt.wantErr {
				return
			}
			rendered := string(output)
			checksValue := `value: "` + tt.checks + `"`
			if !strings.Contains(rendered, checksValue) {
				t.Fatalf("missing CHECKS=%s", tt.checks)
			}
			wantKinds := map[string]int{"Application": 3, "ConfigMap": 1, "NetworkPolicy": 1, "Probe": 1, "PrometheusRule": 1}
			if tt.noPolicy {
				wantKinds["Application"]--
			}
			if tt.noAlerts {
				delete(wantKinds, "PrometheusRule")
			}
			for _, kind := range tt.kinds {
				wantKinds[kind]++
			}
			gotKinds := make(map[string]int)
			for _, doc := range strings.Split(rendered, "\n---") {
				if !strings.Contains(doc, "\nkind: ") {
					continue
				}
				if !strings.Contains(doc, "\n  namespace: nais-system\n") {
					t.Errorf("resource outside nais-system:\n%s", doc)
				}
				for _, line := range strings.Split(doc, "\n") {
					if kind, ok := strings.CutPrefix(line, "kind: "); ok {
						gotKinds[kind]++
					}
				}
				if strings.Contains(doc, "\n  name: contests-noaccess\n") {
					if !strings.Contains(doc, "name: CHECKS\n      value: none") {
						t.Error("negative target must not run service checks")
					}
					for _, configuration := range []string{"uses:", "gcp:", "kafka:", "valkey:", "accessPolicy:"} {
						if strings.Contains(doc, configuration) {
							t.Errorf("negative target must not have %s", configuration)
						}
					}
				}
			}
			for kind, want := range wantKinds {
				if got := gotKinds[kind]; got != want {
					t.Errorf("%s count = %d, want %d", kind, got, want)
				}
				delete(gotKinds, kind)
			}
			if len(gotKinds) != 0 {
				t.Errorf("unexpected resource kinds: %v", gotKinds)
			}
			if !tt.noAlerts && tt.checks != "none" {
				for name := range strings.SplitSeq(tt.checks, ",") {
					if !strings.Contains(rendered, `app="contests", check="`+name+`"`) {
						t.Errorf("missing absent-metric alarm for %s", name)
					}
				}
			}
			if tt.checks == "none" && strings.Contains(rendered, "ContestsCheck") {
				t.Error("network-only installation must not alert on disabled service checks")
			}
			for _, target := range []string{"http://contests/ping", "https://dns.google", "https://1dot1dot1dot1.cloudflare-dns.com", tt.extraTarget} {
				if !strings.Contains(rendered, target) {
					t.Errorf("missing allowed probe %s", target)
				}
			}
			for _, target := range []string{"http://contests-noaccess/ping", "https://google.com"} {
				if got := strings.Contains(rendered, target); got == tt.noPolicy {
					t.Errorf("denied probe %s present = %t", target, got)
				}
			}
			if !tt.noAlerts {
				for _, alert := range []string{"Access policy denied service allowed", "External URL allowed"} {
					if got := strings.Contains(rendered, "alert: "+alert); got == tt.noPolicy {
						t.Errorf("denied-probe alert %s present = %t", alert, got)
					}
				}
				if !strings.Contains(rendered, `job="probe/nais-system/contests-probes"`) {
					t.Error("network alerts must select this release's Probe job")
				}
			}
			if strings.Contains(tt.checks, "postgres") {
				for _, configuration := range []string{"apiVersion: nais.io/v1\nkind: Postgres", "majorVersion: \"18\"", "role: readwrite"} {
					if !strings.Contains(rendered, configuration) {
						t.Errorf("missing Nais Postgres configuration %s", configuration)
					}
				}
			}
			if strings.Contains(tt.checks, "bucket") && !strings.Contains(rendered, `"contests-tenant-env"`) {
				t.Error("existing bucket identity must be preserved")
			}
			for _, removed := range []string{"nais-verification", "bigQueryDatasets", "openSearch:", "azure:", "sqlInstances", "data.nais.io", "http://contests/postgres", "http://contests/kafka"} {
				if strings.Contains(rendered, removed) {
					t.Errorf("removed configuration remains: %s", removed)
				}
			}
		})
	}
}
