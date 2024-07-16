SELECT
    {{ .Ident "resource_version" | .Into .Response.ResourceVersion }},
    {{ .Ident "namespace" | .Into .Response.Key.Namespace }},
    {{ .Ident "group" | .Into .Response.Key.Group }},
    {{ .Ident "resource" | .Into .Response.Key.Resource }},
    {{ .Ident "name" | .Into .Response.Key.Name }},
    {{ .Ident "value" | .Into .Response.Value }},
    {{ .Ident "action" | .Into .Response.Action }}

    FROM {{ .Ident "resource_history" }}
    WHERE 1 = 0
    {{- range $g, $items := .Since }}
    {{- range $r, $rv := $items }}
        OR 
        ( 
            "group" = "{{ $g }}" AND "resource" = "{{ $r }}" AND "resource_version" > {{ $rv }} AND 1 = 1
        )
    {{- end }}
    {{- end }}
    ORDER BY {{ .Ident "resource_version" }} ASC
;
