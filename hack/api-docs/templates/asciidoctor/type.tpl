{{- define "type" -}}
{{- $type := . -}}
{{- if asciidocShouldRenderType $type -}}
{{- if not $type.Markers.hidefromdoc -}}

[id="{{ asciidocTypeID $type | asciidocRenderAnchorID }}"]
==== {{ $type.Name  }}

{{ if $type.IsAlias }}_Underlying type:_ _{{ asciidocRenderTypeLink $type.UnderlyingType  }}_{{ end }}

{{ $type.Doc }}

{{ if $type.Validation -}}
.Validation:
{{- range $type.Validation }}
- {{ . }}
{{- end }}
{{- end }}

{{ if $type.References -}}
.Appears In:
****
{{- range $type.SortedReferences }}
- {{ asciidocRenderTypeLink . }}
{{- end }}
****
{{- end }}

{{ if $type.Members -}}
[cols="20a,50a,15a,15a", options="header"]
|===
| Field | Description | Default | Validation
{{ if $type.GVK -}}
| *`apiVersion`* __string__ | `{{ $type.GVK.Group }}/{{ $type.GVK.Version }}` | |
| *`kind`* __string__ | `{{ $type.GVK.Kind }}` | |
{{ end -}}

{{ range $type.Members -}}
{{ with .Markers.hidefromdoc -}}
{{ else -}}
| *`{{ .Name  }}`* __{{ asciidocRenderType .Type }}__ | {{ template "type_members" . }} | {{ .Default }} | {{ range .Validation -}} {{ asciidocRenderValidation . }} +{{ end }}
{{ end }}
{{ end -}}
|===
{{ end -}}

{{ if $type.EnumValues -}} 
|===
| Field | Description |
{{ range $type.EnumValues -}}
| `{{ .Name }}` | {{ asciidocRenderFieldDoc .Doc }} +
{{ end -}}
|===
{{ end -}}

{{- end -}}
{{- end -}}
{{- end -}}