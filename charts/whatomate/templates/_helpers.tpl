{{- define "whatomate.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "whatomate.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else if contains (include "whatomate.name" .) .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name (include "whatomate.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "whatomate.selectorLabels" -}}
app.kubernetes.io/name: {{ include "whatomate.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "whatomate.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version }}
{{ include "whatomate.selectorLabels" . }}
app.kubernetes.io/version: {{ include "whatomate.tag" . | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "whatomate.tag" -}}
{{- .Values.image.tag | default .Chart.AppVersion }}
{{- end }}

{{- define "whatomate.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- .Values.serviceAccount.name | default (include "whatomate.fullname" .) }}
{{- else }}
{{- .Values.serviceAccount.name | default "default" }}
{{- end }}
{{- end }}

{{/* "database.password" -> WHATOMATE_DATABASE__PASSWORD; levels are joined by a double underscore. */}}
{{- define "whatomate.envName" -}}
{{- printf "WHATOMATE_%s" (. | replace "." "__" | upper) }}
{{- end }}

{{- define "whatomate.secretName" -}}
{{- .Values.secrets.existingSecret | default (printf "%s-secrets" (include "whatomate.fullname" .)) }}
{{- end }}
