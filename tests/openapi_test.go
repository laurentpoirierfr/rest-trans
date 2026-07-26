package tests

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestOpenAPISpec(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/openapi.json")
	if err != nil {
		t.Fatalf("GET /openapi.json failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var spec map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&spec)

	if spec["openapi"] == nil {
		t.Error("Expected openapi version")
	}

	if spec["info"] == nil {
		t.Error("Expected info object")
	}

	if spec["paths"] == nil {
		t.Error("Expected paths object")
	}
}

func TestOpenAPIHasUsersPath(t *testing.T) {
	resp, _ := http.Get(suite.ServerURL + "/openapi.json")
	var spec map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&spec)
	resp.Body.Close()

	paths := spec["paths"].(map[string]interface{})

	if _, ok := paths["/public/users"]; !ok {
		t.Error("Expected /public/users path")
	}
}

func TestOpenAPIHasProjectsPath(t *testing.T) {
	resp, _ := http.Get(suite.ServerURL + "/openapi.json")
	var spec map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&spec)
	resp.Body.Close()

	paths := spec["paths"].(map[string]interface{})

	if _, ok := paths["/public/projects"]; !ok {
		t.Error("Expected /public/projects path")
	}
}

func TestOpenAPIHasRPCPath(t *testing.T) {
	resp, _ := http.Get(suite.ServerURL + "/openapi.json")
	var spec map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&spec)
	resp.Body.Close()

	paths := spec["paths"].(map[string]interface{})

	hasRPC := false
	for path := range paths {
		if pathContains(path, "/rpc/") {
			hasRPC = true
			break
		}
	}

	if !hasRPC {
		t.Error("Expected at least one RPC path")
	}
}

func TestInfoEndpoint(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/info")
	if err != nil {
		t.Fatalf("GET /info failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var info map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&info)

	if info["version"] == nil {
		t.Error("Expected version field")
	}

	if info["tables"] == nil {
		t.Error("Expected tables field")
	}
}

func TestInfoEndpointHasUsers(t *testing.T) {
	resp, _ := http.Get(suite.ServerURL + "/info")
	var info map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&info)
	resp.Body.Close()

	tables := info["tables"].(map[string]interface{})
	publicTables := tables["public"].([]interface{})

	hasUsers := false
	for _, t := range publicTables {
		if t.(string) == "users" {
			hasUsers = true
			break
		}
	}

	if !hasUsers {
		t.Error("Expected users table in /info")
	}
}

func pathContains(path, substr string) bool {
	return len(path) > 0 && len(substr) > 0 && pathContainsImpl(path, substr)
}

func pathContainsImpl(path, substr string) bool {
	for i := 0; i <= len(path)-len(substr); i++ {
		if path[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
