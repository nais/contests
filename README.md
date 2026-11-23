# contests

Continuous verification of Nais network access contracts and Kafka
connectivity. The installation stays in `nais-system`.

## Network contracts

Blackbox exporter probes these targets once per minute:

| Target | Expected result |
| --- | --- |
| `http://contests/ping` | Reachable with explicit application access |
| `http://contests-noaccess/ping` | Unreachable without application access |
| `https://dns.google` | Reachable with explicit external access |
| `https://1dot1dot1dot1.cloudflare-dns.com` | Reachable with explicit external access |
| `https://google.com` | Unreachable without external access |

`contests` and `contests-noaccess` use the same small HTTP server image.
Only `contests` receives Kafka credentials when the Kafka check is enabled.
Network alerts retain their existing five-minute threshold. Additional allowed
targets can be supplied through `blackboxExporter.additionalTargetURLs`.

These are the existing HTTP-based tests, not a complete isolation proof.
Ingress and egress denial are not tested independently, blocked targets have no
positive control, and DNS/TLS/HTTP failures can mask unintended connectivity.
The DNS-provider URLs test HTTPS reachability, not DNS queries to those providers.

## Kafka connectivity

When enabled, blackbox exporter probes `/kafka` once per minute. Each request
loads the current mounted client certificate, private key and CA, then establishes
a TLS connection to every address in `KAFKA_BROKERS`. All connections must succeed
within a shared eight-second deadline.

This checks broker reachability and TLS with the injected client credentials.
It does not send Kafka protocol requests, produce or consume messages, test
topic permissions or provision a topic. Kafka-canary owns messaging tests.

## Metrics and alerts

`/ping` is a process health check. `/metrics` exposes Go/process metrics.
Blackbox exporter's `probe_success` records the network results, with alerts
after five minutes for failed allowed probes or successful denied probes.
The Kafka rule also alerts after five minutes if its probe result is missing.

## Dashboard

[`grafana/contests.json`](grafana/contests.json) is a read-only Git Sync dashboard
for [monitoring.nais.io](https://monitoring.nais.io). Select one tenant's
`*-metrics` datasource and one cluster. Namespace defaults to `nais-system`;
the Helm release defaults to `contests`.

The dashboard shows expected results, history, probe duration, HTTP response
codes, scrape health and sample age. Denied targets invert `probe_success`:
blocked is green, reachable is red. Results older than three minutes or with
failed scrapes are not shown as healthy. This is a display freshness guard,
not an SLO or a change to the alert rules.

No data means unknown; Kafka and denied probes may also be disabled by chart
configuration. Scrape health says whether blackbox exporter answered, not
whether a target was reachable. Denied probes normally take time to fail.
HTTP status for `/kafka` describes the test endpoint, not a Kafka response.

Git Sync reads `grafana/` on the default branch and requires the GitHub App to
have repository access. No Helm dashboard ConfigMap or manual upload is needed.

## Configuration

The chart provisions the test applications, blackbox exporter and monitoring
resources. Fasit enables Kafka where the environment has a Kafka provider and
supplies `contests.kafka.pool`. Kafka is disabled by default outside Fasit:

```sh
helm lint --strict charts/contests
helm template contests charts/contests
```

To render the Kafka check locally, add
`--set contests.kafka.enabled=true --set contests.kafka.pool=test`.
The application exposes `/kafka` only when `KAFKA_BROKERS` is configured.

Resources use the Helm release namespace, `nais-system` in Fasit.
`contests.accessPolicy.enabled=false` disables the negative target, negative
probes and access-policy alerts. `alerts.enabled=false` disables all alert rules.

## Development and delivery

```sh
mise run check
mise run build
mise run local
```

Checks include formatting, tidy module manifests, race-enabled tests, lint,
vulnerability scanning, module checksums and Helm validation. Go tests cover
the HTTP endpoints, TLS connectivity checks and rendered network contracts.

The image is a statically linked Go binary in `scratch`,
running as a non-root user. Builds use the committed `go.mod` and `go.sum`.
The shared `nais/actions` workflow handles checks, builds and artifact publication
for pull requests and pushes to `main` and `simplify`. Branch builds publish a
versioned image and Helm chart without automatic rollout. The chart is published
to `oci://europe-north1-docker.pkg.dev/nais-io/nais/charts/contests`; use the
version from the workflow's Metadata or Build Helm Chart job in Fasit.
Only main deploys through Fasit: ci-nais first, followed by the remaining tenant
targets.
