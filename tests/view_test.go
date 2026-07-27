package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestViewAllowsGET(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/active_users")
	if err != nil {
		t.Fatalf("GET /public/active_users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)

	if len(users) == 0 {
		t.Error("Expected at least 1 user in view")
	}
}

func TestViewAllowsHEAD(t *testing.T) {
	resp, err := http.Head(suite.ServerURL + "/public/active_users")
	if err != nil {
		t.Fatalf("HEAD /public/active_users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

func TestViewRejectsPOST(t *testing.T) {
	body := `{"name": "Test", "email": "test-view@example.com"}`
	req, _ := http.NewRequest("POST", suite.ServerURL+"/public/active_users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /public/active_users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", resp.StatusCode)
	}

	var errResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&errResp)

	if errResp["code"] != "PGRST201" {
		t.Errorf("Expected error code PGRST201, got %v", errResp["code"])
	}
}

func TestViewRejectsPUT(t *testing.T) {
	body := `{"id": 1, "name": "Test", "email": "test-view@example.com"}`
	req, _ := http.NewRequest("PUT", suite.ServerURL+"/public/active_users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /public/active_users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", resp.StatusCode)
	}
}

func TestViewRejectsPATCH(t *testing.T) {
	body := `{"name": "Updated"}`
	req, _ := http.NewRequest("PATCH", suite.ServerURL+"/public/active_users?id=eq.1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /public/active_users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", resp.StatusCode)
	}
}

func TestViewRejectsDELETE(t *testing.T) {
	req, _ := http.NewRequest("DELETE", suite.ServerURL+"/public/active_users?id=eq.1", nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /public/active_users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", resp.StatusCode)
	}
}

func TestViewOptionsHeader(t *testing.T) {
	req, _ := http.NewRequest("OPTIONS", suite.ServerURL+"/public/active_users", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /public/active_users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	allow := resp.Header.Get("Allow")
	if allow != "GET, HEAD, OPTIONS" {
		t.Errorf("Expected 'GET, HEAD, OPTIONS', got '%s'", allow)
	}

	var info map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&info)

	if info["is_view"] != true {
		t.Error("Expected is_view to be true")
	}
}

func TestViewOptionsBody(t *testing.T) {
	resp, _ := http.Get(suite.ServerURL + "/public/active_users")
	resp.Body.Close()

	optsResp, _ := http.Head(suite.ServerURL + "/public/active_users")
	optsResp.Body.Close()

	req, _ := http.NewRequest("OPTIONS", suite.ServerURL+"/public/active_users", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS failed: %v", err)
	}
	defer resp.Body.Close()

	var info map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&info)

	if info["table_name"] != "active_users" {
		t.Errorf("Expected table_name 'active_users', got %v", info["table_name"])
	}

	if info["is_view"] != true {
		t.Error("Expected is_view to be true")
	}

	columns, ok := info["columns"].([]interface{})
	if !ok || len(columns) == 0 {
		t.Error("Expected columns in response")
	}
}

func TestOpenAPIOmitsMutationsForViews(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/openapi.json")
	if err != nil {
		t.Fatalf("GET /openapi.json failed: %v", err)
	}
	defer resp.Body.Close()

	var spec map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&spec)

	paths := spec["paths"].(map[string]interface{})

	viewPath, ok := paths["/public/active_users"]
	if !ok {
		t.Fatal("Expected /public/active_users in OpenAPI spec")
	}

	viewOps := viewPath.(map[string]interface{})

	if _, ok := viewOps["post"]; ok {
		t.Error("View should not have POST operation in OpenAPI")
	}
	if _, ok := viewOps["put"]; ok {
		t.Error("View should not have PUT operation in OpenAPI")
	}
	if _, ok := viewOps["patch"]; ok {
		t.Error("View should not have PATCH operation in OpenAPI")
	}
	if _, ok := viewOps["delete"]; ok {
		t.Error("View should not have DELETE operation in OpenAPI")
	}

	if _, ok := viewOps["get"]; !ok {
		t.Error("View should have GET operation in OpenAPI")
	}
	if _, ok := viewOps["head"]; !ok {
		t.Error("View should have HEAD operation in OpenAPI")
	}
	if _, ok := viewOps["options"]; !ok {
		t.Error("View should have OPTIONS operation in OpenAPI")
	}
}
