{{- /*
Generate name for ServiceAccount
*/}}
{{- define "vault-secrets-exporter.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{- .Values.serviceAccount.name | default (include "vault-secrets-exporter.fullname" .) -}}
{{- else -}}
    {{- .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- /*
Generate Chart full name
*/}}
{{- define "vault-secrets-exporter.fullname" -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

