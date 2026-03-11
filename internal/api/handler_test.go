package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/claudework/network-filter-proxy/internal/api"
	"github.com/claudework/network-filter-proxy/internal/logger"
	"github.com/claudework/network-filter-proxy/internal/proxy"
	"github.com/claudework/network-filter-proxy/internal/rule"
)

func newTestAPI(t *testing.T) (http.Handler, *rule.Store) {
	t.Helper()
	store := rule.NewStore()
	log := logger.New("json", "debug")
	proxyHandler := proxy.NewHandler(store, log)
	h := api.NewHandler(store, log, proxyHandler)
	return h.Routes(), store
}

func TestHealth(t *testing.T) {
	handler, _ := newTestAPI(t)

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want %q", resp["status"], "ok")
	}
}

func TestGetRules_Empty(t *testing.T) {
	handler, _ := newTestAPI(t)

	req := httptest.NewRequest("GET", "/api/v1/rules", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	rules, ok := resp["rules"].(map[string]any)
	if !ok {
		t.Fatal("expected 'rules' to be a map")
	}
	if len(rules) != 0 {
		t.Errorf("expected empty rules, got %d", len(rules))
	}
}

func TestPutRules_Valid(t *testing.T) {
	handler, _ := newTestAPI(t)

	body := `{"entries":[{"host":"api.anthropic.com","port":443}]}`
	req := httptest.NewRequest("PUT", "/api/v1/rules/172.20.0.3", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp["source_ip"] != "172.20.0.3" {
		t.Errorf("source_ip = %v, want %q", resp["source_ip"], "172.20.0.3")
	}
}

func TestPutRules_ValidationError(t *testing.T) {
	handler, _ := newTestAPI(t)

	body := `{"entries":[{"host":"*.*.example.com","port":443}]}`
	req := httptest.NewRequest("PUT", "/api/v1/rules/172.20.0.3", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp["error"] != "validation_error" {
		t.Errorf("error = %v, want %q", resp["error"], "validation_error")
	}
}

func TestDeleteRulesByIP_Exists(t *testing.T) {
	handler, store := newTestAPI(t)

	store.Set("172.20.0.3", []rule.Entry{{Host: "example.com", Port: 443}})

	req := httptest.NewRequest("DELETE", "/api/v1/rules/172.20.0.3", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestDeleteRulesByIP_NotFound(t *testing.T) {
	handler, _ := newTestAPI(t)

	req := httptest.NewRequest("DELETE", "/api/v1/rules/172.20.0.99", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp["error"] != "not_found" {
		t.Errorf("error = %v, want %q", resp["error"], "not_found")
	}
}

func TestDeleteAllRules(t *testing.T) {
	handler, store := newTestAPI(t)

	store.Set("10.0.0.1", []rule.Entry{{Host: "a.com", Port: 443}})
	store.Set("10.0.0.2", []rule.Entry{{Host: "b.com", Port: 443}})

	req := httptest.NewRequest("DELETE", "/api/v1/rules", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if store.Count() != 0 {
		t.Errorf("expected 0 rules after delete all, got %d", store.Count())
	}
}
