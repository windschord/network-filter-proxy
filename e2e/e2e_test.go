package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/claudework/network-filter-proxy/internal/api"
	"github.com/claudework/network-filter-proxy/internal/logger"
	"github.com/claudework/network-filter-proxy/internal/proxy"
	"github.com/claudework/network-filter-proxy/internal/rule"
)

type testEnv struct {
	store        *rule.Store
	proxyHandler *proxy.Handler
	apiHandler   *api.Handler
	proxyServer  *httptest.Server
	apiServer    *httptest.Server
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	log := logger.New("json", "debug")
	store := rule.NewStore()
	proxyHandler := proxy.NewHandler(store, log)
	apiHandler := api.NewHandler(store, log, proxyHandler)

	proxyServer := httptest.NewServer(proxyHandler)
	apiServer := httptest.NewServer(apiHandler.Routes())

	t.Cleanup(func() {
		proxyServer.Close()
		apiServer.Close()
	})

	return &testEnv{
		store:        store,
		proxyHandler: proxyHandler,
		apiHandler:   apiHandler,
		proxyServer:  proxyServer,
		apiServer:    apiServer,
	}
}

func TestE2E_HealthCheck(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.apiServer.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var health map[string]any
	json.NewDecoder(resp.Body).Decode(&health)
	if health["status"] != "ok" {
		t.Errorf("health status = %v, want %q", health["status"], "ok")
	}
}

func TestE2E_RulesCRUD(t *testing.T) {
	env := setupTestEnv(t)
	client := &http.Client{}

	// 1. GET /rules - initially empty
	resp, err := http.Get(env.apiServer.URL + "/api/v1/rules")
	if err != nil {
		t.Fatalf("get rules failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("get rules status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// 2. PUT /rules/10.0.0.1 - create rule
	body := `{"entries":[{"host":"example.com","port":443},{"host":"*.github.com","port":443}]}`
	req, _ := http.NewRequest("PUT", env.apiServer.URL+"/api/v1/rules/10.0.0.1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("put rules failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("put rules status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// 3. GET /rules - should have 1 rule
	resp, err = http.Get(env.apiServer.URL + "/api/v1/rules")
	if err != nil {
		t.Fatalf("get rules failed: %v", err)
	}
	var rulesResp map[string]any
	json.NewDecoder(resp.Body).Decode(&rulesResp)
	resp.Body.Close()
	rules := rulesResp["rules"].(map[string]any)
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}

	// 4. DELETE /rules/10.0.0.1 - delete rule
	req, _ = http.NewRequest("DELETE", env.apiServer.URL+"/api/v1/rules/10.0.0.1", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("delete rules failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete rules status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	// 5. Health check shows 0 rules
	resp, err = http.Get(env.apiServer.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	var health map[string]any
	json.NewDecoder(resp.Body).Decode(&health)
	resp.Body.Close()
	if health["rule_count"].(float64) != 0 {
		t.Errorf("rule_count = %v, want 0", health["rule_count"])
	}
}

func TestE2E_ProxyAllowAndDeny(t *testing.T) {
	env := setupTestEnv(t)

	// Target HTTP server
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "Hello from target")
	}))
	defer target.Close()

	targetURL, _ := url.Parse(target.URL)
	targetHost := targetURL.Hostname()
	targetPort := targetURL.Port()

	proxyURL, _ := url.Parse(env.proxyServer.URL)
	proxyClient := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   5 * time.Second,
	}

	// Step 1: No rules -> should be denied
	resp, err := proxyClient.Get(target.URL)
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("unregistered: status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if reason := resp.Header.Get("X-Filter-Reason"); reason != "no-rules" {
		t.Errorf("X-Filter-Reason = %q, want %q", reason, "no-rules")
	}

	// Step 2: Register rule via API allowing the target
	body := fmt.Sprintf(`{"entries":[{"host":"%s","port":%s}]}`, targetHost, targetPort)
	req, _ := http.NewRequest("PUT", env.apiServer.URL+"/api/v1/rules/127.0.0.1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	apiResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put rules failed: %v", err)
	}
	apiResp.Body.Close()
	if apiResp.StatusCode != http.StatusOK {
		t.Fatalf("put rules status = %d, want %d", apiResp.StatusCode, http.StatusOK)
	}

	// Step 3: Now proxy request should be allowed
	resp2, err := proxyClient.Get(target.URL)
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("allowed: status = %d, want %d", resp2.StatusCode, http.StatusOK)
	}
	respBody, _ := io.ReadAll(resp2.Body)
	if string(respBody) != "Hello from target" {
		t.Errorf("body = %q, want %q", string(respBody), "Hello from target")
	}

	// Step 4: Request to a different target should be denied
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer other.Close()

	resp3, err := proxyClient.Get(other.URL)
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusForbidden {
		t.Errorf("denied: status = %d, want %d", resp3.StatusCode, http.StatusForbidden)
	}
	if reason := resp3.Header.Get("X-Filter-Reason"); reason != "denied" {
		t.Errorf("X-Filter-Reason = %q, want %q", reason, "denied")
	}
}

func TestE2E_ProxyCONNECT_AllowAndDeny(t *testing.T) {
	env := setupTestEnv(t)

	// TLS target server
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "HTTPS OK")
	}))
	defer target.Close()

	targetURL, _ := url.Parse(target.URL)
	targetHost := targetURL.Hostname()

	// Register rule for CONNECT
	body := fmt.Sprintf(`{"entries":[{"host":"%s","port":0}]}`, targetHost)
	req, _ := http.NewRequest("PUT", env.apiServer.URL+"/api/v1/rules/127.0.0.1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	apiResp, _ := http.DefaultClient.Do(req)
	apiResp.Body.Close()

	proxyURL, _ := url.Parse(env.proxyServer.URL)
	proxyClient := &http.Client{
		Transport: &http.Transport{
			Proxy:             http.ProxyURL(proxyURL),
			TLSClientConfig:   target.Client().Transport.(*http.Transport).TLSClientConfig,
			DisableKeepAlives: true,
		},
		Timeout: 5 * time.Second,
	}

	// Allowed CONNECT
	resp, err := proxyClient.Get(target.URL)
	if err != nil {
		t.Fatalf("CONNECT request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("CONNECT allowed: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	respBody, _ := io.ReadAll(resp.Body)
	if string(respBody) != "HTTPS OK" {
		t.Errorf("body = %q, want %q", string(respBody), "HTTPS OK")
	}
}

func TestE2E_ValidationError(t *testing.T) {
	env := setupTestEnv(t)

	body := `{"entries":[{"host":"*.*.bad.com","port":443}]}`
	req, _ := http.NewRequest("PUT", env.apiServer.URL+"/api/v1/rules/10.0.0.1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put rules failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("validation error: status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp map[string]any
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp["error"] != "validation_error" {
		t.Errorf("error = %v, want %q", errResp["error"], "validation_error")
	}
}

func TestE2E_DeleteAllRules(t *testing.T) {
	env := setupTestEnv(t)

	// Add rules
	for _, ip := range []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"} {
		body := `{"entries":[{"host":"example.com","port":443}]}`
		req, _ := http.NewRequest("PUT", env.apiServer.URL+"/api/v1/rules/"+ip, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
	}

	// Verify 3 rules
	resp, _ := http.Get(env.apiServer.URL + "/api/v1/health")
	var health map[string]any
	json.NewDecoder(resp.Body).Decode(&health)
	resp.Body.Close()
	if health["rule_count"].(float64) != 3 {
		t.Fatalf("rule_count = %v, want 3", health["rule_count"])
	}

	// Delete all
	req, _ := http.NewRequest("DELETE", env.apiServer.URL+"/api/v1/rules", nil)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete all: status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	// Verify 0 rules
	resp, _ = http.Get(env.apiServer.URL + "/api/v1/health")
	json.NewDecoder(resp.Body).Decode(&health)
	resp.Body.Close()
	if health["rule_count"].(float64) != 0 {
		t.Errorf("rule_count after delete all = %v, want 0", health["rule_count"])
	}
}

func TestE2E_ActiveConnections(t *testing.T) {
	env := setupTestEnv(t)

	// Initially 0 active connections
	resp, _ := http.Get(env.apiServer.URL + "/api/v1/health")
	var health map[string]any
	json.NewDecoder(resp.Body).Decode(&health)
	resp.Body.Close()
	if health["active_connections"].(float64) != 0 {
		t.Errorf("active_connections = %v, want 0", health["active_connections"])
	}
}

func TestE2E_WildcardMatching(t *testing.T) {
	env := setupTestEnv(t)

	// Create a target server
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	targetURL, _ := url.Parse(target.URL)
	_, targetPortStr, _ := net.SplitHostPort(targetURL.Host)

	// Register wildcard rule for *.0.0.1 (won't match 127.0.0.1, but test port 0 wildcard)
	body := `{"entries":[{"host":"127.0.0.1","port":0}]}`
	req, _ := http.NewRequest("PUT", env.apiServer.URL+"/api/v1/rules/127.0.0.1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	apiResp, _ := http.DefaultClient.Do(req)
	apiResp.Body.Close()

	proxyURL, _ := url.Parse(env.proxyServer.URL)
	proxyClient := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   5 * time.Second,
	}

	// Port 0 wildcard should match any port
	resp, err := proxyClient.Get(target.URL)
	if err != nil {
		t.Fatalf("proxy request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("port 0 wildcard: status = %d (port %s), want %d", resp.StatusCode, targetPortStr, http.StatusOK)
	}
}
