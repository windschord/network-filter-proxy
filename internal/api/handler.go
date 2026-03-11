package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/claudework/network-filter-proxy/internal/proxy"
	"github.com/claudework/network-filter-proxy/internal/rule"
)

type Handler struct {
	store        *rule.Store
	logger       *slog.Logger
	proxyHandler *proxy.Handler
	startTime    time.Time
}

type entryJSON struct {
	Host string `json:"host"`
	Port int    `json:"port,omitempty"`
}

type putRulesRequest struct {
	Entries []entryJSON `json:"entries"`
}

type putRulesResponse struct {
	SourceIP  string      `json:"source_ip"`
	Entries   []entryJSON `json:"entries"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type errorResponse struct {
	Error   string        `json:"error"`
	Message string        `json:"message"`
	Details []errorDetail `json:"details,omitempty"`
}

type errorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type healthResponse struct {
	Status            string `json:"status"`
	UptimeSeconds     int64  `json:"uptime_seconds"`
	ActiveConnections int64  `json:"active_connections"`
	RuleCount         int    `json:"rule_count"`
}

func NewHandler(store *rule.Store, logger *slog.Logger, proxyHandler *proxy.Handler) *Handler {
	return &Handler{
		store:        store,
		logger:       logger,
		proxyHandler: proxyHandler,
		startTime:    time.Now(),
	}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", h.handleHealth)
	mux.HandleFunc("GET /api/v1/rules", h.handleGetRules)
	mux.HandleFunc("PUT /api/v1/rules/{sourceIP}", h.handlePutRules)
	mux.HandleFunc("DELETE /api/v1/rules/{sourceIP}", h.handleDeleteRulesByIP)
	mux.HandleFunc("DELETE /api/v1/rules", h.handleDeleteAllRules)
	return mux
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status:            "ok",
		UptimeSeconds:     int64(time.Since(h.startTime).Seconds()),
		ActiveConnections: h.proxyHandler.ActiveConnections(),
		RuleCount:         h.store.Count(),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleGetRules(w http.ResponseWriter, r *http.Request) {
	all := h.store.All()
	result := make(map[string]any, len(all))
	for ip, rs := range all {
		entries := make([]entryJSON, len(rs.Entries))
		for i, e := range rs.Entries {
			entries[i] = entryJSON{Host: e.Host, Port: e.Port}
		}
		result[ip] = map[string]any{
			"entries":    entries,
			"updated_at": rs.UpdatedAt,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": result})
}

func (h *Handler) handlePutRules(w http.ResponseWriter, r *http.Request) {
	sourceIP := r.PathValue("sourceIP")

	var req putRulesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "invalid_request",
			Message: "failed to parse request body: " + err.Error(),
		})
		return
	}

	var details []errorDetail
	entries := make([]rule.Entry, len(req.Entries))
	for i, e := range req.Entries {
		entry := rule.Entry{Host: e.Host, Port: e.Port}
		if err := rule.ValidateEntry(entry); err != nil {
			details = append(details, errorDetail{
				Field:   fmt.Sprintf("entries[%d].host", i),
				Message: err.Error(),
			})
		}
		entries[i] = entry
	}

	if len(details) > 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "validation_error",
			Message: fmt.Sprintf("invalid host pattern: %s", req.Entries[0].Host),
			Details: details,
		})
		return
	}

	h.store.Set(sourceIP, entries)
	h.logger.Info("rules updated", "operation", "set", "src_ip", sourceIP, "entry_count", len(entries))

	rs, _ := h.store.Get(sourceIP)
	respEntries := make([]entryJSON, len(rs.Entries))
	for i, e := range rs.Entries {
		respEntries[i] = entryJSON{Host: e.Host, Port: e.Port}
	}
	writeJSON(w, http.StatusOK, putRulesResponse{
		SourceIP:  sourceIP,
		Entries:   respEntries,
		UpdatedAt: rs.UpdatedAt,
	})
}

func (h *Handler) handleDeleteRulesByIP(w http.ResponseWriter, r *http.Request) {
	sourceIP := r.PathValue("sourceIP")
	if !h.store.Delete(sourceIP) {
		writeJSON(w, http.StatusNotFound, errorResponse{
			Error:   "not_found",
			Message: fmt.Sprintf("no rules found for source IP: %s", sourceIP),
		})
		return
	}
	h.logger.Info("rules deleted", "operation", "delete", "src_ip", sourceIP)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleDeleteAllRules(w http.ResponseWriter, r *http.Request) {
	h.store.DeleteAll()
	h.logger.Info("all rules deleted", "operation", "delete_all")
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
