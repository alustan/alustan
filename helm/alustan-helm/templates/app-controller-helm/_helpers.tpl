
{{- define "app-controller-helm.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}


{{- define "app-controller-helm.labels" -}}
helm.sh/chart: {{ include "app-controller-helm.chart" . }}
{{ include "app-controller-helm.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app: app-controller-helm
{{- end }}


{{- define "app-controller-helm.selectorLabels" -}}
app.kubernetes.io/name: app-controller-helm
app.kubernetes.io/instance: {{ .Release.Name }}
app: app-controller-helm
{{- end }}


