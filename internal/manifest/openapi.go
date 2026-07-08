package manifest

// OpenAPI — manifest'ten OWUI'nin okuyabilecegi OpenAPI 3.1 dokumani. mcpo'nun urettigi yuzeyi
// elle sunar: her tool = POST /<tool>, requestBody = filtre parametreleri. OWUI tool-server olarak kaydeder.
// (Eskiden toolserver.BuildOpenAPI; connector'a tasindi ki registry cok-connector'u desteklesin.)
func (m *Manifest) OpenAPI(label string) map[string]any {
	paths := map[string]any{}
	for i := range m.Tools {
		t := &m.Tools[i]
		props := map[string]any{}
		var required []string
		for _, f := range t.Filters {
			props[f.Name] = map[string]any{
				"type":        "string", // model daima string gonderebilir; tip-tespiti onboarding'de
				"description": f.Field + " " + f.Op,
			}
			if f.Required {
				required = append(required, f.Name)
			}
		}
		schema := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			schema["required"] = required
		}
		desc := t.Description
		if desc == "" {
			desc = t.Name
		}
		paths["/"+t.Name] = map[string]any{
			"post": map[string]any{
				"operationId": t.Name,
				"summary":     desc,
				"description": desc + " (salt-okunur).",
				"requestBody": map[string]any{
					"required": len(required) > 0,
					"content": map[string]any{
						"application/json": map[string]any{"schema": schema},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "sonuc",
						"content": map[string]any{
							"application/json": map[string]any{"schema": map[string]any{"type": "object"}},
						},
					},
				},
			},
		}
	}
	title := m.Label
	if title == "" {
		title = m.Name
	}
	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       title,
			"version":     "1.0.0",
			"description": "Chimera Masha DB connector (salt-okunur) — " + m.ERPKind,
		},
		"servers": []any{map[string]any{"url": "/" + label}},
		"paths":   paths,
	}
}
