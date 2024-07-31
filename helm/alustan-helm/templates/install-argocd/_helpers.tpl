
{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "install-argocd.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "install-argocd.labels" -}}
helm.sh/chart: {{ include "install-argocd.chart" . }}
{{ include "install-argocd.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}

{{- end }}

{{/*
Selector labels
*/}}
{{- define "install-argocd.selectorLabels" -}}
app.kubernetes.io/name: install-argocd
app.kubernetes.io/instance: {{ .Release.Name }}

{{- end }}


