
{{- define "service-controller-helm.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}


{{- define "service-controller-helm.labels" -}}
helm.sh/chart: {{ include "service-controller-helm.chart" . }}
{{ include "service-controller-helm.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app: service-controller-helm
{{- end }}


{{- define "service-controller-helm.selectorLabels" -}}
app.kubernetes.io/name: service-controller-helm
app.kubernetes.io/instance: {{ .Release.Name }}
app: service-controller-helm
{{- end }}


