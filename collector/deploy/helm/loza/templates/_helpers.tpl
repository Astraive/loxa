{{/*
Expand the name of the chart.
*/}}
{{- define "loza.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "loza.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "loza.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "loza.labels" -}}
helm.sh/chart: {{ include "loza.chart" . }}
{{ include "loza.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "loza.selectorLabels" -}}
app.kubernetes.io/name: {{ include "loza.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Service account name
*/}}
{{- define "loza.serviceAccountName" -}}
{{- if .Values.global.serviceAccount.name }}
{{- .Values.global.serviceAccount.name }}
{{- else }}
{{- include "loza.fullname" . }}
{{- end }}
{{- end }}

{{/*
Return the collector image
*/}}
{{- define "loza.collector.image" -}}
{{- if .Values.collector.image.digest }}
{{- .Values.global.imageRegistry }}/{{ .Values.collector.image.repository | default "loza" }}@{{ .Values.collector.image.digest }}
{{- else }}
{{- if .Values.collector.image.repository }}
{{- .Values.global.imageRegistry }}/{{ .Values.collector.image.repository }}:{{ .Values.collector.image.tag | default "latest" }}
{{- else }}
{{- .Values.global.imageRegistry }}/loza:{{ .Values.collector.image.tag | default "latest" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Return the worker image
*/}}
{{- define "loza.worker.image" -}}
{{- if .Values.worker.image.digest }}
{{- .Values.global.imageRegistry }}/{{ .Values.worker.image.repository | default "loza-worker" }}@{{ .Values.worker.image.digest }}
{{- else }}
{{- if .Values.worker.image.repository }}
{{- .Values.global.imageRegistry }}/{{ .Values.worker.image.repository }}:{{ .Values.worker.image.tag | default "latest" }}
{{- else }}
{{- .Values.global.imageRegistry }}/loza-worker:{{ .Values.worker.image.tag | default "latest" }}
{{- end }}
{{- end }}
{{- end }}
