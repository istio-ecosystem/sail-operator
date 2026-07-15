{{- define "gvDetails" -}}
{{- $gv := . -}}

## {{ $gv.GroupVersionString }}

{{ $gv.Doc }}

{{- if $gv.Kinds  }}
### Resource Types
{{- range $gv.SortedKinds }}
{{- $type := $gv.TypeForKind . }}
{{- if $type.GVK }}
- [{{ $type.Name }}](#{{ $type.Name | lower }}-{{ $type.GVK.Version }})
{{- else }}
- [{{ $type.Name }}](#{{ $type.Name | lower }})
{{- end }}
{{- end }}
{{ end }}

{{ range $gv.SortedTypes }}
{{ template "type" . }}
{{ end }}

{{- end -}}
