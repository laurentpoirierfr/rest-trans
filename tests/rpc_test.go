package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/laurentpoirierfr/rest-trans/tests/testutil"
)

func TestRPCGetUserProfile(t *testing.T) {
	suite.SetupTest(t)

	email := testutil.UniqueEmail("rpc-profile")
	createBody := fmt.Sprintf(`{"name": "RPC Profile User", "email": "%s"}`, email)
	createReq, _ := http.NewRequest("POST", suite.ServerURL+"/public/users", bytes.NewBufferString(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, _ := http.DefaultClient.Do(createReq)
	createResp.Body.Close()

	getResp, _ := http.Get(suite.ServerURL + "/public/users?email=eq." + email + "&_select=id")
	var users []map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&users)
	getResp.Body.Close()

	if len(users) == 0 {
		t.Fatal("Failed to create user for RPC test")
	}

	userID := int(users[0]["id"].(float64))

	body := fmt.Sprintf(`{"p_user_id": %d}`, userID)
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

	if result[0]["user_name"] != "RPC Profile User" {
		t.Errorf("Expected user_name 'RPC Profile User', got %v", result[0]["user_name"])
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
	suite.SetupTest(t)

	title := testutil.UniqueName("RPC Project")
	body := fmt.Sprintf(`{
		"p_title": "%s",
		"p_author_id": 1,
		"p_tasks": [
			{"title": "%s-Task 1", "task_order": 1},
			{"title": "%s-Task 2", "task_order": 2}
		]
	}`, title, title, title)
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
	suite.SetupTest(t)

	email := testutil.UniqueEmail("rpc-task")
	createBody := fmt.Sprintf(`{"name": "RPC Task Project", "email": "%s"}`, email)
	createReq, _ := http.NewRequest("POST", suite.ServerURL+"/public/users", bytes.NewBufferString(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, _ := http.DefaultClient.Do(createReq)
	createResp.Body.Close()

	getResp, _ := http.Get(suite.ServerURL + "/public/users?email=eq." + email + "&_select=id")
	var users []map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&users)
	getResp.Body.Close()

	if len(users) == 0 {
		t.Fatal("Failed to create user for RPC task test")
	}

	userID := int(users[0]["id"].(float64))

	projBody := fmt.Sprintf(`{"p_title": "Task Project", "p_author_id": %d, "p_tasks": [{"title": "Bump Task", "task_order": 1}]}`, userID)
	projResp, _ := http.Post(
		suite.ServerURL+"/public/rpc/create_project_with_tasks",
		"application/json",
		bytes.NewBufferString(projBody),
	)
	var projResult []map[string]interface{}
	json.NewDecoder(projResp.Body).Decode(&projResult)
	projResp.Body.Close()

	if len(projResult) == 0 {
		t.Fatal("Failed to create project for task bump test")
	}

	taskID := 1

	body := fmt.Sprintf(`{"p_task_id": %d}`, taskID)
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
