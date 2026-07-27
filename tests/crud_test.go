package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/laurentpoirierfr/rest-trans/tests/testutil"
)

var suite *testutil.TestSuite

func TestMain(m *testing.M) {
	suite = testutil.SetupSuite()
	code := m.Run()
	testutil.TeardownAll()
	os.Exit(code)
}

func TestGETUsers(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/users")
	if err != nil {
		t.Fatalf("GET /public/users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)

	if len(users) < 3 {
		t.Errorf("Expected at least 3 users, got %d", len(users))
	}

	if users[0]["name"] == nil {
		t.Error("Expected name field")
	}
}

func TestHEADUsers(t *testing.T) {
	resp, err := http.Head(suite.ServerURL + "/public/users")
	if err != nil {
		t.Fatalf("HEAD /public/users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

func TestPOSTUser(t *testing.T) {
	suite.SetupTest(t)

	email := testutil.UniqueEmail("post")
	body := fmt.Sprintf(`{"name": "Test Post User", "email": "%s"}`, email)
	req, _ := http.NewRequest("POST", suite.ServerURL+"/public/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /public/users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}

	getResp, _ := http.Get(suite.ServerURL + "/public/users?email=eq." + email)
	var users []map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&users)
	getResp.Body.Close()

	if len(users) != 1 {
		t.Fatalf("Expected 1 user, got %d", len(users))
	}
	if users[0]["name"] != "Test Post User" {
		t.Errorf("Expected name 'Test Post User', got %v", users[0]["name"])
	}
}

func TestPOSTUsersBatch(t *testing.T) {
	suite.SetupTest(t)

	suffix := testutil.UniqueSuffix()
	body := fmt.Sprintf(`[{"name": "Batch User 1-%s", "email": "batch1-%s@example.com"}, {"name": "Batch User 2-%s", "email": "batch2-%s@example.com"}]`, suffix, suffix, suffix, suffix)
	req, _ := http.NewRequest("POST", suite.ServerURL+"/public/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /public/users batch failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}
}

func TestPUTUser(t *testing.T) {
	suite.SetupTest(t)

	email := testutil.UniqueEmail("put")
	createBody := fmt.Sprintf(`{"name": "Put Target", "email": "%s"}`, email)
	createReq, _ := http.NewRequest("POST", suite.ServerURL+"/public/users", bytes.NewBufferString(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, _ := http.DefaultClient.Do(createReq)
	createResp.Body.Close()

	getResp, _ := http.Get(suite.ServerURL + "/public/users?email=eq." + email + "&_select=id")
	var users []map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&users)
	getResp.Body.Close()

	if len(users) == 0 {
		t.Fatal("Failed to create user for PUT test")
	}

	id := int(users[0]["id"].(float64))

	body := fmt.Sprintf(`{"id": %d, "name": "Put Updated", "email": "%s"}`, id, email)
	req, _ := http.NewRequest("PUT", suite.ServerURL+"/public/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /public/users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 200 or 201, got %d", resp.StatusCode)
	}
}

func TestPATCHUser(t *testing.T) {
	suite.SetupTest(t)

	email := testutil.UniqueEmail("patch")
	createReq, _ := http.NewRequest("POST", suite.ServerURL+"/public/users", bytes.NewBufferString(fmt.Sprintf(`{"name": "Patch Target", "email": "%s"}`, email)))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, _ := http.DefaultClient.Do(createReq)
	createResp.Body.Close()

	getResp, _ := http.Get(suite.ServerURL + "/public/users?email=eq." + email + "&_select=id")
	var users []map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&users)
	getResp.Body.Close()

	if len(users) == 0 {
		t.Fatal("Failed to create user for PATCH test")
	}

	id := int(users[0]["id"].(float64))

	body := `{"name": "Patch Modified"}`
	req, _ := http.NewRequest("PATCH", suite.ServerURL+"/public/users?id=eq."+strconv.Itoa(id), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /public/users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected 200 or 204, got %d", resp.StatusCode)
	}

	verifyResp, _ := http.Get(suite.ServerURL + "/public/users?id=eq." + strconv.Itoa(id) + "&_select=name")
	var updated []map[string]interface{}
	json.NewDecoder(verifyResp.Body).Decode(&updated)
	verifyResp.Body.Close()

	if len(updated) == 0 || updated[0]["name"] != "Patch Modified" {
		t.Error("PATCH did not update the name")
	}
}

func TestDELETEUser(t *testing.T) {
	suite.SetupTest(t)

	email := testutil.UniqueEmail("delete")
	createReq, _ := http.NewRequest("POST", suite.ServerURL+"/public/users", bytes.NewBufferString(fmt.Sprintf(`{"name": "To Delete", "email": "%s"}`, email)))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, _ := http.DefaultClient.Do(createReq)
	createResp.Body.Close()

	getResp, _ := http.Get(suite.ServerURL + "/public/users?email=eq." + email + "&_select=id")
	var users []map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&users)
	getResp.Body.Close()

	if len(users) == 0 {
		t.Fatal("Failed to create user for DELETE test")
	}

	id := int(users[0]["id"].(float64))

	req, _ := http.NewRequest("DELETE", suite.ServerURL+"/public/users?id=eq."+strconv.Itoa(id), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /public/users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", resp.StatusCode)
	}

	verifyResp, _ := http.Get(suite.ServerURL + "/public/users?id=eq." + strconv.Itoa(id))
	var remaining []map[string]interface{}
	json.NewDecoder(verifyResp.Body).Decode(&remaining)
	verifyResp.Body.Close()

	if len(remaining) != 0 {
		t.Error("DELETE did not remove the user")
	}
}

func TestOPTIONSUser(t *testing.T) {
	req, _ := http.NewRequest("OPTIONS", suite.ServerURL+"/public/users", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /public/users failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}
