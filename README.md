# contests

Continuous verification of Nais network access contracts and four platform
services. The existing installation stays in `nais-system`.

## Network contracts

Blackbox exporter probes these targets once per minute:

| Target | Expected result |
| --- | --- |
| `http://contests/ping` | Reachable with explicit application access |
| `http://contests-noaccess/ping` | Unreachable without application access |
| `https://dns.google` | Reachable with explicit external access |
| `https://1dot1dot1dot1.cloudflare-dns.com` | Reachable with explicit external access |
| `https://google.com` | Unreachable without external access |

`contests-noaccess` uses the same image with `CHECKS=none`: it serves health and
metrics endpoints without service credentials or dependency checks. Network
alerts retain their existing five-minute threshold. Additional allowed targets
can be supplied through `blackboxExporter.additionalTargetURLs`.

These are the existing HTTP-based tests, not a complete isolation proof.
Ingress and egress denial are not tested independently, blocked targets have no
positive control, and DNS/TLS/HTTP failures can mask unintended connectivity.
The DNS-provider URLs test HTTPS reachability, not DNS queries to those providers.

## Platform services

| Check | Verifies | Provisioning |
| --- | --- | --- |
| Postgres | TLS client authentication and `SELECT 1` | Nais Postgres 18, 100m CPU, 256Mi memory, 5Gi disk per instance |
| Kafka | Connect to every broker, produce and consume one message | One topic with read/write access for contests |
| Bucket | Write, read back and delete a small object | Bucket using platform defaults and workload identity |
| Valkey | Set, get and delete a unique key with a one-minute TTL | SingleNode, 1GB (smallest supported), persistence disabled |

Each check runs immediately, then once per minute, with a ten-second deadline.
Checks run independently. A failed service does not prevent the others from
reporting. Credentials are loaded for each run, so new connections use current
mounted certificates. Bucket and Valkey cleanup failures also fail the check.

## Metrics and alerts

`/ping` is a process health check, not a dependency health check. `/metrics`
exposes the usual Go/process metrics and three gauges labeled by `check`:

- `contests_check_success`: 1 on success, 0 on failure.
- `contests_check_last_run_timestamp_seconds`: last completed check.
- `contests_check_duration_seconds`: duration of that check.

Two service rules alert after five minutes: a check is failing, or a configured check
has not reported for three minutes / its metric is absent. Missing metrics
also cover an unavailable application or broken metrics collection.

## Configuration

All four checks are enabled by default. Nais injects service credentials;
the chart supplies only the bucket name, Kafka topic and selected checks.
`CHECKS` defaults to `postgres,kafka,bucket,valkey` for local runs.
Use `CHECKS=none` to serve only health and metrics endpoints.

Postgres uses `spec.uses.postgres` with `role: readwrite` and `READWRITE_PGURL`.
There are no schema migrations, tables or admin credentials. An application
that does run migrations should request the default `admin` role, use `PG*`
for migrations and `READWRITE_PG*` for runtime traffic.

Valkey uses `VALKEY_URI_CONTESTS`, `VALKEY_USERNAME_CONTESTS` and
`VALKEY_PASSWORD_CONTESTS`. The `valkeys://` URI is adapted to the Redis client's
`rediss://` scheme; TLS is required.

Fasit supplies `contests.kafka.pool`. Kafka and Valkey are disabled by computed
defaults in environments without the corresponding provider; checks can also
be disabled individually in Fasit. Helm validation outside Fasit needs a pool:

```sh
helm lint --strict charts/contests --set contests.kafka.pool=test
helm template contests charts/contests --set contests.kafka.pool=test
```

Resources use the Helm release namespace, `nais-system` in Fasit. The Kafka
topic and existing bucket name are preserved; no namespace migration is needed.
Turning off all four service checks leaves the network checks running.
`contests.accessPolicy.enabled=false` disables the negative target, negative
probes and access-policy alerts. `alerts.enabled=false` disables all alert rules.

The new Postgres replaces the old Cloud SQL and legacy Postgres configuration;
there is no data migration. Review deletion protection and retention for old
resources before upgrading. BigQuery, OpenSearch, Azure, HTTP service-check
endpoints and the public ingress are no longer part of contests. Network probes
remain in blackbox exporter; service checks use the metrics described above.

## Development and delivery

```sh
mise run check
mise run build
mise run local
```

Checks include formatting, tidy module manifests, race-enabled tests, lint,
vulnerability scanning, module checksums and Helm validation. Go tests cover
the scheduler and rendered chart as well as the service check behavior.

For the Postgres TLS integration test, provide `READWRITE_PGURL` for a test
database with certificate authentication and a runtime user without schema
creation privileges:

```sh
go test -tags=integration ./internal/database
```

The image is a statically linked Go binary and CA certificates in `scratch`,
running as a non-root user. Builds use the committed `go.mod` and `go.sum`.
The shared `nais/actions` workflow handles checks, builds and artifact publication
for both pull requests and main. Only main deploys through Fasit: ci-nais first,
followed by the remaining tenant targets.
