{{- define "contests.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | quote }}
app.kubernetes.io/name: contests
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
team: nais
{{- end }}

{{- define "contests.checks" -}}
{{- $checks := list -}}
{{- range $name := list "postgres" "kafka" "bucket" "valkey" -}}
{{- if (index $.Values.contests $name).enabled -}}
{{- $checks = append $checks $name -}}
{{- end -}}
{{- end -}}
{{- if not $checks -}}
none
{{- else -}}
{{- join "," $checks -}}
{{- end -}}
{{- end }}

{{- define "contests.bucketName" -}}
{{- printf "contests-%s-%s" .Values.fasit.tenant.name .Values.fasit.env.name -}}
{{- end }}
