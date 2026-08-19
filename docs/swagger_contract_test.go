package docs_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/verbeux-ai/whatsmiau/docs"
	"github.com/verbeux-ai/whatsmiau/server/routes"
	"sigs.k8s.io/yaml"
)

type swaggerDocument struct {
	Swagger string                          `json:"swagger"`
	Base    string                          `json:"basePath"`
	Paths   map[string]map[string]operation `json:"paths"`
}

type operation struct {
	OperationID string                     `json:"operationId"`
	Security    []map[string][]interface{} `json:"security"`
	Responses   map[string]json.RawMessage `json:"responses"`
	Description string                     `json:"description"`
}

func TestSwaggerCoversEveryDocumentedRoute(t *testing.T) {
	var document swaggerDocument
	if err := json.Unmarshal(docs.SwaggerJSON, &document); err != nil {
		t.Fatalf("parse embedded Swagger JSON: %v", err)
	}

	if document.Swagger != "2.0" || document.Base != "/" {
		t.Fatalf("unexpected Swagger metadata: version=%q basePath=%q", document.Swagger, document.Base)
	}

	operations := append(routes.DocumentedV1Operations(), routes.DocumentedDocumentationOperations()...)
	if got, want := len(routes.DocumentedV1Operations()), 102; got != want {
		t.Fatalf("unexpected /v1 operation inventory size: got %d, want %d", got, want)
	}
	if got, want := len(operations), 106; got != want {
		t.Fatalf("unexpected full operation inventory size: got %d, want %d", got, want)
	}

	seenIDs := make(map[string]string, len(operations))
	for _, expected := range operations {
		method := strings.ToLower(expected.Method)
		path, ok := document.Paths[expected.Path]
		if !ok {
			t.Fatalf("Swagger is missing %s %s", expected.Method, expected.Path)
		}
		actual, ok := path[method]
		if !ok {
			t.Fatalf("Swagger is missing %s %s", expected.Method, expected.Path)
		}
		if actual.OperationID == "" {
			t.Fatalf("%s %s has no operationId", expected.Method, expected.Path)
		}
		if other, exists := seenIDs[actual.OperationID]; exists {
			t.Fatalf("duplicate operationId %q for %s and %s %s", actual.OperationID, other, expected.Method, expected.Path)
		}
		seenIDs[actual.OperationID] = fmt.Sprintf("%s %s", expected.Method, expected.Path)

		if strings.HasPrefix(expected.Path, "/v1/") || expected.Path == "/v1" {
			if len(actual.Security) != 1 || actual.Security[0]["ApiKeyAuth"] == nil {
				t.Fatalf("%s %s is missing ApiKeyAuth", expected.Method, expected.Path)
			}
			if _, ok := actual.Responses["401"]; !ok {
				t.Fatalf("%s %s is missing a 401 contract", expected.Method, expected.Path)
			}
		}
	}

	for path, methods := range document.Paths {
		for method, operation := range methods {
			if operation.OperationID == "" {
				t.Fatalf("Swagger operation %s %s has no operationId", strings.ToUpper(method), path)
			}
		}
	}
}

func TestSwaggerDocumentsCallWebSocketAndWireCasing(t *testing.T) {
	var raw map[string]any
	if err := json.Unmarshal(docs.SwaggerJSON, &raw); err != nil {
		t.Fatalf("parse embedded Swagger JSON: %v", err)
	}

	paths := raw["paths"].(map[string]any)
	audio := paths["/v1/instance/{instance}/calls/{callID}/audio"].(map[string]any)["get"].(map[string]any)
	if !strings.Contains(audio["description"].(string), "3,840 bytes") {
		t.Fatal("call audio contract does not describe PCM frame size")
	}
	if _, ok := audio["responses"].(map[string]any)["101"]; !ok {
		t.Fatal("call audio contract does not document WebSocket upgrade")
	}

	definitions := raw["definitions"].(map[string]any)
	offer := definitions["dto.CallOfferResponse"].(map[string]any)["properties"].(map[string]any)
	if _, ok := offer["ID"]; !ok {
		t.Fatal("call offer contract must preserve the ID compatibility key")
	}
	if _, ok := offer["Recipient"]; !ok {
		t.Fatal("call offer contract must preserve the Recipient compatibility key")
	}
}

func TestSwaggerYAMLMatchesTheEmbeddedJSONContract(t *testing.T) {
	jsonFromYAML, err := yaml.YAMLToJSON(docs.SwaggerYAML)
	if err != nil {
		t.Fatalf("parse embedded Swagger YAML: %v", err)
	}

	var document swaggerDocument
	if err := json.Unmarshal(jsonFromYAML, &document); err != nil {
		t.Fatalf("decode Swagger YAML: %v", err)
	}
	if document.Swagger != "2.0" || len(document.Paths) == 0 {
		t.Fatalf("invalid Swagger YAML contract: version=%q paths=%d", document.Swagger, len(document.Paths))
	}
}
