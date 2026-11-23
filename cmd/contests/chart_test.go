package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestChartRendersNetworkContracts(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		extra       []string
		noPolicy    bool
		noAlerts    bool
		kafka       bool
		extraTarget string
	}{
		{name: "defaults"},
		{name: "no denied probes", extra: []string{"contests.accessPolicy.enabled=false"}, noPolicy: true},
		{name: "no alerts", extra: []string{"alerts.enabled=false"}, noAlerts: true},
		{name: "additional target", extra: []string{"blackboxExporter.additionalTargetURLs[0]=https://example.com/health"}, extraTarget: "https://example.com/health"},
		{name: "Kafka enabled", extra: []string{"contests.kafka.enabled=true", "contests.kafka.pool=test"}, kafka: true},
		{name: "Kafka without alerts", extra: []string{"contests.kafka.enabled=true", "contests.kafka.pool=test", "alerts.enabled=false"}, kafka: true, noAlerts: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"template", "contests", "../../charts/contests", "--namespace", "nais-system"}
			for _, setting := range tt.extra {
				args = append(args, "--set", setting)
			}
			output, err := exec.CommandContext(t.Context(), "helm", args...).CombinedOutput()
			if err != nil {
				t.Fatalf("helm template: %v\n%s", err, output)
			}
			rendered := string(output)
			wantKinds := map[string]int{"Application": 3, "ConfigMap": 1, "NetworkPolicy": 1, "Probe": 1, "PrometheusRule": 1}
			if tt.noPolicy {
				wantKinds["Application"]--
			}
			if tt.kafka {
				wantKinds["Probe"]++
			}
			if tt.noAlerts {
				delete(wantKinds, "PrometheusRule")
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
					for _, configuration := range []string{"accessPolicy:", "kafka:"} {
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
			for _, configuration := range []string{"http://contests/kafka", "\n  kafka:\n", "http_kafka:"} {
				if got := strings.Contains(rendered, configuration); got != tt.kafka {
					t.Errorf("Kafka configuration %q present = %t, want %t", configuration, got, tt.kafka)
				}
			}
			if got := strings.Contains(rendered, "alert: Kafka connectivity test failing"); got != (tt.kafka && !tt.noAlerts) {
				t.Errorf("Kafka alert present = %t", got)
			}
			if tt.kafka && !tt.noAlerts {
				if !strings.Contains(rendered, `job="probe/nais-system/contests-kafka-probe"`) || !strings.Contains(rendered, "or absent(probe_success{") {
					t.Error("Kafka alert must detect failed and missing results from this release's Probe job")
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
		})
	}
}
