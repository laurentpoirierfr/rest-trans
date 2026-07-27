package tests

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func insertFTSData(t *testing.T) {
	t.Helper()
	suffix := time.Now().Format("0102150405")

	articles := []struct {
		title string
		body  string
	}{
		{"Introduction to Go", "Go is a statically typed compiled language designed at Google"},
		{"PostgreSQL Full Text Search", "PostgreSQL provides full text search support with tsvector and tsquery"},
		{"Building REST APIs", "REST APIs are a way to expose data over HTTP with CRUD operations"},
	}

	for _, a := range articles {
		_, err := suite.DB.Exec(
			`INSERT INTO articles (title, body) VALUES ($1, to_tsvector('english', $2))`,
			a.title+"-"+suffix,
			a.body,
		)
		if err != nil {
			t.Fatalf("Failed to insert article: %v", err)
		}
	}

	prefix := "%-" + suffix
	t.Cleanup(func() {
		suite.DB.Exec("DELETE FROM articles WHERE title LIKE $1", prefix)
	})
}

func TestFtsBasic(t *testing.T) {
	insertFTSData(t)

	resp, err := http.Get(suite.ServerURL + "/public/articles?_fts=body.Go")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var articles []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&articles)

	if len(articles) == 0 {
		t.Error("Expected at least 1 article matching 'Go'")
	}

	for _, a := range articles {
		if _, ok := a["_rank"]; !ok {
			t.Error("Expected _rank field in response")
		}
	}
}

func TestFtsMultiWord(t *testing.T) {
	insertFTSData(t)

	resp, err := http.Get(suite.ServerURL + "/public/articles?_fts=body.PostgreSQL+search")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var articles []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&articles)

	if len(articles) == 0 {
		t.Error("Expected at least 1 article matching 'PostgreSQL search'")
	}
}

func TestFtsRanking(t *testing.T) {
	insertFTSData(t)

	resp, err := http.Get(suite.ServerURL + "/public/articles?_fts=body.language&_order=_rank.desc")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var articles []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&articles)

	if len(articles) < 2 {
		t.Skip("Need at least 2 articles for ranking test")
	}

	firstRank := articles[0]["_rank"].(float64)
	secondRank := articles[1]["_rank"].(float64)

	if firstRank < secondRank {
		t.Errorf("Expected first article to have higher rank: %f < %f", firstRank, secondRank)
	}
}

func TestFtsInvalidColumn(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/articles?_fts=noncol.test")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", resp.StatusCode)
	}
}

func TestFtsInvalidFormat(t *testing.T) {
	resp, err := http.Get(suite.ServerURL + "/public/articles?_fts=nodot")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", resp.StatusCode)
	}
}

func TestFtsNoMatch(t *testing.T) {
	insertFTSData(t)

	resp, err := http.Get(suite.ServerURL + "/public/articles?_fts=body.zzznonexistent")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var articles []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&articles)

	if len(articles) != 0 {
		t.Errorf("Expected 0 articles, got %d", len(articles))
	}
}
