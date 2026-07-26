package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestRPCGetUserProfile(t *testing.T) {
	body := `{"p_user_id": 1}`
	resp, err := http.Post(
		suite.ServerURL+"/public/rpc/get_user_profile",
		"application/json",
		bytes.NewBufferString(body),
	)
	if err != nil {
		t.Fatalf("POST /rpc/get_user_profile failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var result []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result) != 1 {
		t.Errorf("Expected 1 row, got %d", len(result))
	}

	if result[0]["user_name"] != "Alice Updated" {
		t.Errorf("Expected user_name 'Alice Updated', got %v", result[0]["user_name"])
	}
}

func TestRPCSearchProjects(t *testing.T) {
	body := `{"p_search": "E-Commerce", "p_limit": 10}`
	resp, err := http.Post(
		suite.ServerURL+"/public/rpc/search_projects",
		"application/json",
		bytes.NewBufferString(body),
	)
	if err != nil {
		t.Fatalf("POST /rpc/search_projects failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var result []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result) < 1 {
		t.Error("Expected at least 1 project")
	}
}

func TestRPCGetStats(t *testing.T) {
	resp, err := http.Post(
		suite.ServerURL+"/public/rpc/get_stats",
		"application/json",
		bytes.NewBufferString(`{}`),
	)
	if err != nil {
		t.Fatalf("POST /rpc/get_stats failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var result []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result) != 1 {
		t.Errorf("Expected 1 row, got %d", len(result))
	}

	if result[0]["total_users"] == nil {
		t.Error("Expected total_users field")
	}
}

func TestRPCCreateProjectWithTasks(t *testing.T) {
	body := `{
		"p_title": "RPC Test Project",
		"p_author_id": 1,
		"p_tasks": [
			{"title": "Task 1", "task_order": 1},
			{"title": "Task 2", "task_order": 2}
		]
	}`
	resp, err := http.Post(
		suite.ServerURL+"/public/rpc/create_project_with_tasks",
		"application/json",
		bytes.NewBufferString(body),
	)
	if err != nil {
		t.Fatalf("POST /rpc/create_project_with_tasks failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var result []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result) != 1 {
		t.Errorf("Expected 1 row, got %d", len(result))
	}

	if result[0]["tasks_created"] != float64(2) {
		t.Errorf("Expected 2 tasks created, got %v", result[0]["tasks_created"])
	}
}

func TestRPCBumpTaskPriority(t *testing.T) {
	body := `{"p_task_id": 1}`
	resp, err := http.Post(
		suite.ServerURL+"/public/rpc/bump_task_priority",
		"application/json",
		bytes.NewBufferString(body),
	)
	if err != nil {
		t.Fatalf("POST /rpc/bump_task_priority failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var result []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result) != 1 {
		t.Errorf("Expected 1 row, got %d", len(result))
	}

	if result[0]["new_priority"] == nil {
		t.Error("Expected new_priority field")
	}
}
