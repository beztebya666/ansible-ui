{{- define "ansible-ui.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "ansible-ui.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "ansible-ui.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "ansible-ui.labels" -}}
app.kubernetes.io/name: {{ include "ansible-ui.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "ansible-ui.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ansible-ui.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "ansible-ui.secretName" -}}
{{- if .Values.existingSecret -}}
{{- .Values.existingSecret -}}
{{- else -}}
{{- printf "%s-secrets" (include "ansible-ui.fullname" .) -}}
{{- end -}}
{{- end -}}

{{- define "ansible-ui.databaseURL" -}}
{{- if .Values.postgres.enabled -}}
postgres://{{ .Values.postgres.user }}:{{ .Values.postgres.password }}@{{ include "ansible-ui.fullname" . }}-postgres:5432/{{ .Values.postgres.database }}?sslmode=disable
{{- else -}}
{{- .Values.externalDatabase.url -}}
{{- end -}}
{{- end -}}

{{- define "ansible-ui.apiImage" -}}
{{- printf "%s/%s:%s" .Values.image.registry .Values.image.apiRepository .Values.image.tag -}}
{{- end -}}

{{- define "ansible-ui.runnerImage" -}}
{{- printf "%s/%s:%s" .Values.image.registry .Values.image.runnerRepository .Values.image.tag -}}
{{- end -}}
