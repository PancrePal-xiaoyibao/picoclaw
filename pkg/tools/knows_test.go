package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewKnowsTools_RequiresConfig(t *testing.T) {
	_, err := NewKnowsTools(KnowsToolOptions{})
	if err == nil {
		t.Fatal("expected error when knows config is incomplete")
	}
}

func TestKnowsAISearch_UsesDefaultDataScope(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/knows/ai_search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Fatalf("unexpected x-api-key: %s", got)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}

		scopeRaw, ok := payload["data_scope"].([]interface{})
		if !ok {
			t.Fatalf("data_scope missing or wrong type: %#v", payload["data_scope"])
		}
		if len(scopeRaw) != 2 {
			t.Fatalf("expected 2 data_scope values, got %d", len(scopeRaw))
		}
		if scopeRaw[0] != "GUIDE" || scopeRaw[1] != "PAPER" {
			t.Fatalf("unexpected data_scope values: %#v", scopeRaw)
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"question_id": "q-1",
				"evidences": []map[string]interface{}{
					{"id": "ev-1", "type": "PAPER"},
				},
			},
		})
	}))
	defer server.Close()

	tools, err := NewKnowsTools(KnowsToolOptions{
		APIKey:           "test-key",
		APIBaseURL:       server.URL,
		DefaultDataScope: []string{"GUIDE", "PAPER"},
		RequestTimeout:   10 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewKnowsTools() error = %v", err)
	}

	tool := findToolByName(tools, "knows_ai_search")
	if tool == nil {
		t.Fatal("knows_ai_search tool not found")
	}

	result := tool.Execute(context.Background(), map[string]interface{}{
		"question": "What is latest adjuvant therapy?",
	})
	if result.IsError {
		t.Fatalf("tool execution failed: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, `"question_id":"q-1"`) {
		t.Fatalf("unexpected tool result: %s", result.ForLLM)
	}
}

func TestKnowsListInterpretation_UsesCompatibleEndpoint(t *testing.T) {
	t.Parallel()

	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/knows/list_interpretion" {
			called = true
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"items": []interface{}{},
			},
		})
	}))
	defer server.Close()

	tools, err := NewKnowsTools(KnowsToolOptions{
		APIKey:         "test-key",
		APIBaseURL:     server.URL,
		RequestTimeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewKnowsTools() error = %v", err)
	}

	tool := findToolByName(tools, "knows_list_interpretation")
	if tool == nil {
		t.Fatal("knows_list_interpretation tool not found")
	}

	result := tool.Execute(context.Background(), map[string]interface{}{})
	if result.IsError {
		t.Fatalf("tool execution failed: %s", result.ForLLM)
	}
	if !called {
		t.Fatal("expected /knows/list_interpretion endpoint to be called")
	}
}

func TestKnowsBatchGetEvidenceDetails(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	calls := map[string]int{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls[r.URL.Path]++
		mu.Unlock()

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"path": r.URL.Path,
			},
		})
	}))
	defer server.Close()

	tools, err := NewKnowsTools(KnowsToolOptions{
		APIKey:           "test-key",
		APIBaseURL:       server.URL,
		BatchConcurrency: 2,
		RequestTimeout:   10 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewKnowsTools() error = %v", err)
	}

	tool := findToolByName(tools, "knows_batch_get_evidence_details")
	if tool == nil {
		t.Fatal("knows_batch_get_evidence_details tool not found")
	}

	result := tool.Execute(context.Background(), map[string]interface{}{
		"evidences": []interface{}{
			map[string]interface{}{"evidence_id": "paper-1", "type": "PAPER"},
			map[string]interface{}{"evidence_id": "guide-1", "type": "GUIDE"},
		},
		"translate_to_chinese": true,
	})
	if result.IsError {
		t.Fatalf("tool execution failed: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, `"status":"success"`) {
		t.Fatalf("unexpected batch result: %s", result.ForLLM)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls["/knows/evidence/get_paper_en"] == 0 {
		t.Fatal("expected /knows/evidence/get_paper_en to be called")
	}
	if calls["/knows/evidence/get_guide"] == 0 {
		t.Fatal("expected /knows/evidence/get_guide to be called")
	}
}

func findToolByName(all []Tool, name string) Tool {
	for _, tool := range all {
		if tool.Name() == name {
			return tool
		}
	}
	return nil
}
