package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"sigs.k8s.io/yaml"
)

func main() {
	content, err := os.ReadFile("docs/swagger.json")
	if err != nil {
		panic(fmt.Errorf("read swagger JSON: %w", err))
	}

	var spec map[string]any
	if err := json.Unmarshal(content, &spec); err != nil {
		panic(fmt.Errorf("parse swagger JSON: %w", err))
	}

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		panic("swagger JSON has no paths object")
	}

	pathNames := make([]string, 0, len(paths))
	for path := range paths {
		pathNames = append(pathNames, path)
	}
	sort.Strings(pathNames)

	for _, path := range pathNames {
		pathItem, ok := paths[path].(map[string]any)
		if !ok {
			continue
		}
		methods := make([]string, 0, len(pathItem))
		for method := range pathItem {
			if isHTTPMethod(method) {
				methods = append(methods, method)
			}
		}
		sort.Strings(methods)

		for _, method := range methods {
			operation, ok := pathItem[method].(map[string]any)
			if !ok {
				continue
			}
			operation["operationId"] = operationID(method, path)
			if strings.HasPrefix(path, "/v1/") || path == "/v1" {
				operation["security"] = []any{map[string]any{"ApiKeyAuth": []any{}}}
				responses, ok := operation["responses"].(map[string]any)
				if !ok {
					responses = make(map[string]any)
					operation["responses"] = responses
				}
				if _, exists := responses["401"]; !exists {
					responses["401"] = map[string]any{
						"description": "Unauthorized",
						"schema":      map[string]any{"$ref": "#/definitions/utils.AuthenticationErrorResponse"},
					}
				}
			}
		}
	}

	jsonContent, err := json.MarshalIndent(spec, "", "    ")
	if err != nil {
		panic(fmt.Errorf("marshal swagger JSON: %w", err))
	}
	if err := os.WriteFile("docs/swagger.json", append(jsonContent, '\n'), 0o644); err != nil {
		panic(fmt.Errorf("write swagger JSON: %w", err))
	}

	yamlContent, err := yaml.JSONToYAML(jsonContent)
	if err != nil {
		panic(fmt.Errorf("marshal swagger YAML: %w", err))
	}
	if err := os.WriteFile("docs/swagger.yaml", yamlContent, 0o644); err != nil {
		panic(fmt.Errorf("write swagger YAML: %w", err))
	}
}

func isHTTPMethod(method string) bool {
	switch method {
	case "get", "post", "put", "patch", "delete", "head", "options":
		return true
	default:
		return false
	}
}

func operationID(method, path string) string {
	parts := []string{strings.ToLower(method)}
	for _, segment := range strings.Split(strings.Trim(path, "/"), "/") {
		if segment == "" {
			continue
		}
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			parts = append(parts, "by", segment[1:len(segment)-1])
			continue
		}
		parts = append(parts, segment)
	}

	var result strings.Builder
	for _, part := range parts {
		capitalize := true
		for _, r := range part {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				capitalize = true
				continue
			}
			if result.Len() == 0 {
				result.WriteRune(unicode.ToLower(r))
				capitalize = false
				continue
			}
			if capitalize {
				result.WriteRune(unicode.ToUpper(r))
				capitalize = false
			} else {
				result.WriteRune(r)
			}
		}
	}
	return result.String()
}
