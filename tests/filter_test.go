package tests

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSelectColumns(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/users?_select=id,name")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)

	if len(users) == 0 {
		t.Fatal("Expected at least 1 user")
	}

	if _, ok := users[0]["id"]; !ok {
		t.Error("Expected 'id' field")
	}
	if _, ok := users[0]["name"]; !ok {
		t.Error("Expected 'name' field")
	}
	if _, ok := users[0]["email"]; ok {
		t.Error("Should not have 'email' field when not selected")
	}
}

func TestFilterEq(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/users?email=eq.bob@example.com")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)

	if len(users) != 1 {
		t.Errorf("Expected 1 user, got %d", len(users))
	}
	if users[0]["name"] != "Bob Martin" {
		t.Errorf("Expected 'Bob Martin', got %v", users[0]["name"])
	}
}

func TestFilterNeq(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/users?email=neq.bob@example.com")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)

	for _, u := range users {
		if u["email"] == "bob@example.com" {
			t.Error("Should not contain 'bob@example.com'")
		}
	}
}

func TestFilterGt(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/users?id=gt.1")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)

	if len(users) < 2 {
		t.Errorf("Expected at least 2 users, got %d", len(users))
	}
}

func TestFilterLike(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/users?name=like.%Bob%")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)

	if len(users) < 1 {
		t.Error("Expected at least 1 user")
	}
}

func TestFilterIn(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/users?id=in.(1,2)")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)

	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}
}

func TestFilterIsNull(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/projects?author_id=is.null")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var projects []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&projects)

	if len(projects) < 1 {
		t.Error("Expected at least 1 project with null author_id")
	}
}

func TestFilterOr(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/users?_or=(id.eq.1,id.eq.3)")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)

	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}
}

func TestFilterAnd(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/users?_and=(id.gte.1,id.lte.2)")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)

	if len(users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(users))
	}
}

func TestOrderAsc(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/projects?_order=title.asc")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var projects []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&projects)

	if len(projects) < 2 {
		t.Fatalf("Expected at least 2 projects, got %d", len(projects))
	}

	for i := 1; i < len(projects); i++ {
		if projects[i]["title"].(string) < projects[i-1]["title"].(string) {
			t.Error("Projects not in ascending order")
			break
		}
	}
}

func TestOrderDesc(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/projects?_order=title.desc")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var projects []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&projects)

	if len(projects) < 2 {
		t.Fatalf("Expected at least 2 projects, got %d", len(projects))
	}

	for i := 1; i < len(projects); i++ {
		if projects[i]["title"].(string) > projects[i-1]["title"].(string) {
			t.Error("Projects not in descending order")
			break
		}
	}
}

func TestLimitOffset(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/users?_limit=1&_offset=1")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)

	if len(users) != 1 {
		t.Errorf("Expected 1 user, got %d", len(users))
	}
}

func TestContentRange(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/users?_limit=2&_offset=0&_count=exact")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	contentRange := resp.Header.Get("Content-Range")
	if contentRange == "" {
		t.Error("Expected Content-Range header")
	}
}

func TestResourceEmbedding(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/projects?_select=title,project_tasks(title)")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var projects []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&projects)

	if len(projects) == 0 {
		t.Fatal("Expected at least 1 project")
	}

	hasTasks := false
	for _, p := range projects {
		if _, ok := p["project_tasks"]; ok {
			hasTasks = true
			break
		}
	}
	if !hasTasks {
		t.Error("Expected project_tasks in response")
	}
}

func TestFilterJsonb(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/projects?settings=cs.{\"is_public\":true}")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var projects []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&projects)

	if len(projects) < 1 {
		t.Error("Expected at least 1 project with is_public=true")
	}
}
