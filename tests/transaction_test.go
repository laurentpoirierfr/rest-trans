package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/laurentpoirierfr/rest-trans/tests/testutil"
)

func TestTransactionStart(t *testing.T) {
	resp, err := http.Post(
		suite.ServerURL+"/public/transactions",
		"application/json",
		bytes.NewBufferString(`{}`),
	)
	if err != nil {
		t.Fatalf("POST /public/transactions failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["tx"] == nil {
		t.Error("Expected tx ID in response")
	}
}

func TestTransactionStage(t *testing.T) {
	suite.SetupTest(t)

	startResp, _ := http.Post(
		suite.ServerURL+"/public/transactions",
		"application/json",
		bytes.NewBufferString(`{}`),
	)
	var startResult map[string]interface{}
	json.NewDecoder(startResp.Body).Decode(&startResult)
	startResp.Body.Close()

	txID := startResult["tx"].(string)

	email := testutil.UniqueEmail("tx-stage")
	body := fmt.Sprintf(`{"name": "Tx User", "email": "%s"}`, email)
	req, _ := http.NewRequest(
		"POST",
		suite.ServerURL+"/public/users",
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization-Transaction", txID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /public/users with tx failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("Expected 202, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "pending" {
		t.Errorf("Expected status 'pending', got %v", result["status"])
	}
}

func TestTransactionGetStatus(t *testing.T) {
	startResp, _ := http.Post(
		suite.ServerURL+"/public/transactions",
		"application/json",
		bytes.NewBufferString(`{}`),
	)
	var startResult map[string]interface{}
	json.NewDecoder(startResp.Body).Decode(&startResult)
	startResp.Body.Close()

	txID := startResult["tx"].(string)

	resp, err := http.Get(suite.ServerURL + "/public/transactions/" + txID)
	if err != nil {
		t.Fatalf("GET /public/transactions/%s failed: %v", txID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "pending" {
		t.Errorf("Expected status 'pending', got %v", result["status"])
	}
}

func TestTransactionCommit(t *testing.T) {
	suite.SetupTest(t)

	startResp, _ := http.Post(
		suite.ServerURL+"/public/transactions",
		"application/json",
		bytes.NewBufferString(`{}`),
	)
	var startResult map[string]interface{}
	json.NewDecoder(startResp.Body).Decode(&startResult)
	startResp.Body.Close()

	txID := startResult["tx"].(string)

	email := testutil.UniqueEmail("tx-commit")
	body := fmt.Sprintf(`{"name": "Committed User", "email": "%s"}`, email)
	req, _ := http.NewRequest(
		"POST",
		suite.ServerURL+"/public/users",
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization-Transaction", txID)
	http.DefaultClient.Do(req)

	commitResp, err := http.Post(
		suite.ServerURL+"/public/transactions/"+txID+"/commit",
		"application/json",
		bytes.NewBufferString(`{}`),
	)
	if err != nil {
		t.Fatalf("POST /public/transactions/%s/commit failed: %v", txID, err)
	}
	defer commitResp.Body.Close()

	if commitResp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", commitResp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(commitResp.Body).Decode(&result)

	if result["status"] != "committed" {
		t.Errorf("Expected status 'committed', got %v", result["status"])
	}

	getResp, _ := http.Get(suite.ServerURL + "/public/users?email=eq." + email)
	var users []map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&users)
	getResp.Body.Close()

	if len(users) != 1 {
		t.Errorf("Expected committed user to exist, got %d users", len(users))
	}
}

func TestTransactionRollback(t *testing.T) {
	suite.SetupTest(t)

	startResp, _ := http.Post(
		suite.ServerURL+"/public/transactions",
		"application/json",
		bytes.NewBufferString(`{}`),
	)
	var startResult map[string]interface{}
	json.NewDecoder(startResp.Body).Decode(&startResult)
	startResp.Body.Close()

	txID := startResult["tx"].(string)

	email := testutil.UniqueEmail("tx-rollback")
	body := fmt.Sprintf(`{"name": "Rolled Back User", "email": "%s"}`, email)
	req, _ := http.NewRequest(
		"POST",
		suite.ServerURL+"/public/users",
		bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization-Transaction", txID)
	http.DefaultClient.Do(req)

	rollbackResp, err := http.Post(
		suite.ServerURL+"/public/transactions/"+txID+"/rollback",
		"application/json",
		bytes.NewBufferString(`{}`),
	)
	if err != nil {
		t.Fatalf("POST /public/transactions/%s/rollback failed: %v", txID, err)
	}
	defer rollbackResp.Body.Close()

	if rollbackResp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", rollbackResp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(rollbackResp.Body).Decode(&result)

	if result["status"] != "rolled_back" {
		t.Errorf("Expected status 'rolled_back', got %v", result["status"])
	}

	getResp, _ := http.Get(suite.ServerURL + "/public/users?email=eq." + email)
	var users []map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&users)
	getResp.Body.Close()

	if len(users) != 0 {
		t.Errorf("Expected no users after rollback, got %d users", len(users))
	}
}

func TestTransactionList(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/transactions")
	if err != nil {
		t.Fatalf("GET /public/transactions failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var result []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if len(result) == 0 {
		t.Error("Expected at least 1 transaction")
	}
}
