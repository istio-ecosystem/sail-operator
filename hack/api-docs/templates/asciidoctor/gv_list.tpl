{{- define "gvList" -}}
{{- $groupVersions := . -}}

[id="{p}-api-reference"]
== API Reference

.Packages
{{- range $groupVersions }}
- {{ asciidocRenderGVLink . }}
{{- end }}

{{ range $groupVersions }}
{{ template "gvDetails" . }}
{{ end }}

{{- end -}}