package api

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpenAPISpecIsValidAndDocumentsEveryRoute(t *testing.T) {
	spec := OpenAPISpec()
	var document struct {
		OpenAPI    string                    `yaml:"openapi"`
		Paths      map[string]map[string]any `yaml:"paths"`
		Components struct {
			Schemas         map[string]any `yaml:"schemas"`
			Responses       map[string]any `yaml:"responses"`
			SecuritySchemes map[string]any `yaml:"securitySchemes"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(spec, &document); err != nil {
		t.Fatalf("parse OpenAPI YAML: %v", err)
	}
	if document.OpenAPI != "3.0.3" {
		t.Fatalf("OpenAPI version = %q", document.OpenAPI)
	}
	want := map[string][]string{
		"/api/v1/sessions/guest":          {"post"},
		"/api/v1/sessions/refresh":        {"post"},
		"/api/v1/sessions/current":        {"get", "delete"},
		"/api/v1/games":                   {"get", "post"},
		"/api/v1/games/{gameId}":          {"get", "delete"},
		"/api/v1/games/{gameId}/commands": {"post"},
		"/api/v1/catalog":                 {"get"},
		"/api/v1/catalog/cards/{cardId}":  {"get"},
		"/health/live":                    {"get"},
		"/health/ready":                   {"get"},
		"/openapi.yaml":                   {"get"},
		"/docs/":                          {"get"},
	}
	for path, methods := range want {
		operations, ok := document.Paths[path]
		if !ok {
			t.Errorf("OpenAPI does not document %s", path)
			continue
		}
		for _, method := range methods {
			operation, ok := operations[method]
			if !ok {
				t.Errorf("OpenAPI does not document %s %s", method, path)
				continue
			}
			operationMap, ok := operation.(map[string]any)
			if !ok {
				t.Errorf("OpenAPI operation %s %s is malformed", method, path)
				continue
			}
			if responses, ok := operationMap["responses"].(map[string]any); !ok || len(responses) == 0 {
				t.Errorf("OpenAPI operation %s %s has no responses", method, path)
			}
		}
	}
	if len(document.Components.Schemas) < 20 || len(document.Components.Responses) < 8 {
		t.Fatalf("OpenAPI components are incomplete: schemas=%d responses=%d", len(document.Components.Schemas), len(document.Components.Responses))
	}
	for _, scheme := range []string{"BearerAuth", "RefreshCookie"} {
		if _, ok := document.Components.SecuritySchemes[scheme]; !ok {
			t.Errorf("missing security scheme %s", scheme)
		}
	}
	for _, schema := range []string{"DifficultyDefinition", "VictoryModifiers"} {
		if _, ok := document.Components.Schemas[schema]; !ok {
			t.Errorf("missing schema %s", schema)
		}
	}
}
