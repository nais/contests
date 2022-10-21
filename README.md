# contests
(stripped down version of https://github.com/nais/testapp)

## services

simple go binary that exposes the following services

* `/ping(?delay=<duration>)` (returns "pong\n" and HTTP 200. Valid durations include 10s, 6m, 9h etc, and will delay the response accordingly)
* `/internal/metrics` (prints Prometheus metrics)) 
* `/version` (prints running version of testapp binary) 
* `/connect` (performs a HTTP GET to the URL configured in `$CONNECT_URL` and prints the result. Ignores certs)
* `/database/test` (test that write/read works for database)
* `/bucket/test` (test that write/read works for bucket)
* `/bigquery/test` (test that write/read works for bigquery)
* `/kafka/test` (test that connects to aiven kafka)

## options
```
      --app-name string              application name (used when having several instances of application running in same namespace) (default "testapp")
      --bind-address string          ip:port where http requests are served (default ":8080")
      --bucket-name string           name of bucket used with /{read,write}bucket
      --bucket-object-name string    name of bucket object used with /{read,write}bucket (default "test")
      --connect-url string           URL to connect to with /connect (default "https://google.com")
      --db-hostname string           database hostname (default "localhost")
      --db-name string               database name (default "testapp")
      --db-password string           database password
      --db-user string               database username (default "testapp")
      --bigqueryName                 bigquery dataset name (default "bigqueryname")
	  --bigqueryTableName            bigquery table name (default "bigquerytablename")
      --graceful-shutdown-wait int   when receiving interrupt signal, it will wait this amount of seconds before shutting down server
      --ping-response string         what to respond when pinged (default "pong\n")

```
* options are available as env vars where dash is replaced by underscore and letters capitalized. E.g. `--connect-url` is available as `CONNECT_URL` env var.