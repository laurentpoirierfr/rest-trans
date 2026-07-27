package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func insertRollbackTestData(t *testing.T) (int, string) {
	t.Helper()
	suffix := time.Now().UnixNano()

	var id int
	err := suite.DB.QueryRow(
		`INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id`,
		fmt.Sprintf("rollback-user-%d", suffix),
		fmt.Sprintf("rollback-%d@example.com", suffix),
	).Scan(&id)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	t.Cleanup(func() {
		suite.DB.Exec("DELETE FROM users WHERE id = $1", id)
	})

	return id, fmt.Sprintf("rollback-user-%d", suffix)
}

func TestPostCommitRollbackUpdate(t *testing.T) {
	userID, originalName := insertRollbackTestData(t)

	txResp, _ := http.Post(suite.ServerURL+"/public/transactions", "application/json", nil)
	var txBody map[string]string
	json.NewDecoder(txResp.Body).Decode(&txBody)
	txID := txBody["tx"]
	txResp.Body.Close()

	body := fmt.Sprintf(`{"name":"updated-name-%d"}`, time.Now().UnixNano())
	req, _ := http.NewRequest("PATCH", suite.ServerURL+"/public/users?id=eq."+itoa(userID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization-Transaction", txID)
	stagedResp, _ := http.DefaultClient.Do(req)
	stagedResp.Body.Close()

	commitReq, _ := http.NewRequest("POST", suite.ServerURL+"/public/transactions/"+txID+"/commit", nil)
	commitResp, _ := http.DefaultClient.Do(commitReq)
	commitResp.Body.Close()

	if commitResp.StatusCode != http.StatusOK {
		t.Fatalf("Commit failed: %d", commitResp.StatusCode)
	}

	var currentName string
	err := suite.DB.QueryRow("SELECT name FROM users WHERE id = $1", userID).Scan(&currentName)
	if err != nil {
		t.Fatalf("Failed to read current name: %v", err)
	}
	if currentName == originalName {
		t.Error("Name should have been updated after commit")
	}

	rollbackReq, _ := http.NewRequest("POST", suite.ServerURL+"/public/transactions/"+txID+"/rollback", nil)
	rollbackResp, _ := http.DefaultClient.Do(rollbackReq)
	rollbackResp.Body.Close()

	if rollbackResp.StatusCode != http.StatusOK {
		t.Fatalf("Rollback failed: %d", rollbackResp.StatusCode)
	}

	var restoredName string
	err = suite.DB.QueryRow("SELECT name FROM users WHERE id = $1", userID).Scan(&restoredName)
	if err != nil {
		t.Fatalf("Failed to read restored name: %v", err)
	}
	if restoredName != originalName {
		t.Errorf("Name should be restored to '%s', got '%s'", originalName, restoredName)
	}
}

func TestPostCommitRollbackInsert(t *testing.T) {
	txResp, _ := http.Post(suite.ServerURL+"/public/transactions", "application/json", nil)
	var txBody map[string]string
	json.NewDecoder(txResp.Body).Decode(&txBody)
	txID := txBody["tx"]
	txResp.Body.Close()

	suffix := time.Now().UnixNano()
	email := fmt.Sprintf("new-%d@example.com", suffix)
	body := fmt.Sprintf(`{"name":"new-user-%d","email":"%s"}`, suffix, email)
	req, _ := http.NewRequest("POST", suite.ServerURL+"/public/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization-Transaction", txID)
	stagedResp, _ := http.DefaultClient.Do(req)
	stagedResp.Body.Close()

	commitReq, _ := http.NewRequest("POST", suite.ServerURL+"/public/transactions/"+txID+"/commit", nil)
	commitResp, _ := http.DefaultClient.Do(commitReq)
	commitResp.Body.Close()

	if commitResp.StatusCode != http.StatusOK {
		t.Fatalf("Commit failed: %d", commitResp.StatusCode)
	}

	var newUserID int
	err := suite.DB.QueryRow("SELECT id FROM users WHERE email = $1", email).Scan(&newUserID)
	if err != nil {
		t.Fatalf("Failed to find new user after commit: %v", err)
	}

	rollbackReq, _ := http.NewRequest("POST", suite.ServerURL+"/public/transactions/"+txID+"/rollback", nil)
	rollbackResp, _ := http.DefaultClient.Do(rollbackReq)
	rollbackResp.Body.Close()

	if rollbackResp.StatusCode != http.StatusOK {
		t.Fatalf("Rollback failed: %d", rollbackResp.StatusCode)
	}

	var count int
	err = suite.DB.QueryRow("SELECT COUNT(*) FROM users WHERE id = $1", newUserID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check deleted user: %v", err)
	}
	if count != 0 {
		t.Error("User should have been deleted after rollback of INSERT")
	}
}

func TestPostCommitRollbackDelete(t *testing.T) {
	userID, originalName := insertRollbackTestData(t)

	txResp, _ := http.Post(suite.ServerURL+"/public/transactions", "application/json", nil)
	var txBody map[string]string
	json.NewDecoder(txResp.Body).Decode(&txBody)
	txID := txBody["tx"]
	txResp.Body.Close()

	req, _ := http.NewRequest("DELETE", suite.ServerURL+"/public/users?id=eq."+itoa(userID), nil)
	req.Header.Set("Authorization-Transaction", txID)
	stagedResp, _ := http.DefaultClient.Do(req)
	stagedResp.Body.Close()

	commitReq, _ := http.NewRequest("POST", suite.ServerURL+"/public/transactions/"+txID+"/commit", nil)
	commitResp, _ := http.DefaultClient.Do(commitReq)
	commitResp.Body.Close()

	var count int
	err := suite.DB.QueryRow("SELECT COUNT(*) FROM users WHERE id = $1", userID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check deleted user: %v", err)
	}
	if count != 0 {
		t.Error("User should have been deleted after commit")
	}

	rollbackReq, _ := http.NewRequest("POST", suite.ServerURL+"/public/transactions/"+txID+"/rollback", nil)
	rollbackResp, _ := http.DefaultClient.Do(rollbackReq)
	rollbackResp.Body.Close()

	if rollbackResp.StatusCode != http.StatusOK {
		t.Fatalf("Rollback failed: %d", rollbackResp.StatusCode)
	}

	var restoredName string
	err = suite.DB.QueryRow("SELECT name FROM users WHERE id = $1", userID).Scan(&restoredName)
	if err != nil {
		t.Fatalf("Failed to read restored user: %v", err)
	}
	if restoredName != originalName {
		t.Errorf("User should be restored with name '%s', got '%s'", originalName, restoredName)
	}
}

func TestPostCommitRollbackPending(t *testing.T) {
	txResp, _ := http.Post(suite.ServerURL+"/public/transactions", "application/json", nil)
	var txBody map[string]string
	json.NewDecoder(txResp.Body).Decode(&txBody)
	txID := txBody["tx"]
	txResp.Body.Close()

	body := `{"name":"test-user","email":"test-pending@example.com"}`
	req, _ := http.NewRequest("POST", suite.ServerURL+"/public/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization-Transaction", txID)
	stagedResp, _ := http.DefaultClient.Do(req)
	stagedResp.Body.Close()

	rollbackReq, _ := http.NewRequest("POST", suite.ServerURL+"/public/transactions/"+txID+"/rollback", nil)
	rollbackResp, _ := http.DefaultClient.Do(rollbackReq)
	rollbackResp.Body.Close()

	if rollbackResp.StatusCode != http.StatusOK {
		t.Fatalf("Rollback failed: %d", rollbackResp.StatusCode)
	}

	var txStatus string
	err := suite.DB.QueryRow("SELECT status FROM rest_transactions WHERE id = $1", txID).Scan(&txStatus)
	if err != nil {
		t.Fatalf("Failed to get transaction status: %v", err)
	}
	if txStatus != "rolled_back" {
		t.Errorf("Transaction status should be 'rolled_back', got '%s'", txStatus)
	}
}

func TestPostCommitRollbackConflict(t *testing.T) {
	userID, _ := insertRollbackTestData(t)

	txResp, _ := http.Post(suite.ServerURL+"/public/transactions", "application/json", nil)
	var txBody map[string]string
	json.NewDecoder(txResp.Body).Decode(&txBody)
	txID := txBody["tx"]
	txResp.Body.Close()

	body := fmt.Sprintf(`{"name":"will-be-conflicted-%d"}`, time.Now().UnixNano())
	req, _ := http.NewRequest("PATCH", suite.ServerURL+"/public/users?id=eq."+itoa(userID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization-Transaction", txID)
	stagedResp, _ := http.DefaultClient.Do(req)
	stagedResp.Body.Close()

	commitReq, _ := http.NewRequest("POST", suite.ServerURL+"/public/transactions/"+txID+"/commit", nil)
	commitResp, _ := http.DefaultClient.Do(commitReq)
	commitResp.Body.Close()

	otherBody := fmt.Sprintf(`{"name":"modified-by-other-%d"}`, time.Now().UnixNano())
	otherReq, _ := http.NewRequest("PATCH", suite.ServerURL+"/public/users?id=eq."+itoa(userID), bytes.NewBufferString(otherBody))
	otherReq.Header.Set("Content-Type", "application/json")
	otherResp, _ := http.DefaultClient.Do(otherReq)
	otherResp.Body.Close()

	rollbackReq, _ := http.NewRequest("POST", suite.ServerURL+"/public/transactions/"+txID+"/rollback", nil)
	rollbackResp, _ := http.DefaultClient.Do(rollbackReq)
	rollbackResp.Body.Close()

	if rollbackResp.StatusCode != http.StatusConflict {
		t.Errorf("Expected 409 Conflict, got %d", rollbackResp.StatusCode)
	}
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
