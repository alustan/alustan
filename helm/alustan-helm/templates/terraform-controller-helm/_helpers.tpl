
{{- define "terraform-controller-helm.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}


{{- define "terraform-controller-helm.labels" -}}
helm.sh/chart: {{ include "terraform-controller-helm.chart" . }}
{{ include "terraform-controller-helm.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app: terraform-controller-helm

{{- end }}


{{- define "terraform-controller-helm.selectorLabels" -}}
app.kubernetes.io/name: terraform-controller-helm
app.kubernetes.io/instance: {{ .Release.Name }}
app: terraform-controller-helm
 
{{- end }}



