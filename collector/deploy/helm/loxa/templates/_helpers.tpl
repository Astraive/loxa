{{/*
Expand the name of the chart.
*/}}
{{- define "loxa.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "loxa.fullname" -}}
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
{{- define "loxa.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "loxa.labels" -}}
helm.sh/chart: {{ include "loxa.chart" . }}
{{ include "loxa.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "loxa.selectorLabels" -}}
app.kubernetes.io/name: {{ include "loxa.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Service account name
*/}}
{{- define "loxa.serviceAccountName" -}}
{{- if .Values.global.serviceAccount.name }}
{{- .Values.global.serviceAccount.name }}
{{- else }}
{{- include "loxa.fullname" . }}
{{- end }}
{{- end }}

{{/*
Return the collector image
*/}}
{{- define "loxa.collector.image" -}}
{{- if .Values.collector.image.repository }}
{{- .Values.global.imageRegistry }}/{{ .Values.collector.image.repository }}:{{ .Values.collector.image.tag | default "latest" }}
{{- else }}
{{- .Values.global.imageRegistry }}/loxa-collector:{{ .Values.collector.image.tag | default "latest" }}
{{- end }}
{{- end }}

{{/*
Return the worker image
*/}}
{{- define "loxa.worker.image" -}}
{{- if .Values.worker.image.repository }}
{{- .Values.global.imageRegistry }}/{{ .Values.worker.image.repository }}:{{ .Values.worker.image.tag | default "latest" }}
{{- else }}
{{- .Values.global.imageRegistry }}/loxa-worker:{{ .Values.worker.image.tag | default "latest" }}
{{- end }}
{{- end }}