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

// Handler is the Management API handler.
type Handler struct {
	store        *rule.Store
	logger       *slog.Logger
	proxyHandler *proxy.Handler
	startTime    time.Time
}

// EntryJSON represents a whitelist entry in API requests/responses.
type EntryJSON struct {
	Host string `json:"host" example:"api.anthropic.com"`
	Port int    `json:"port,omitempty" example:"443"`
}

// PutRulesRequest is the request body for PUT /api/v1/rules/{sourceIP}.
type PutRulesRequest struct {
	Entries []EntryJSON `json:"entries"`
}

// PutRulesResponse is the response body for PUT /api/v1/rules/{sourceIP}.
type PutRulesResponse struct {
	SourceIP  string      `json:"source_ip" example:"172.20.0.3"`
	Entries   []EntryJSON `json:"entries"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// ErrorResponse represents an API error.
type ErrorResponse struct {
	Error   string        `json:"error" example:"validation_error"`
	Message string        `json:"message" example:"invalid host pattern"`
	Details []ErrorDetail `json:"details,omitempty"`
}

// ErrorDetail provides field-level error information.
type ErrorDetail struct {
	Field   string `json:"field" example:"entries[0].host"`
	Message string `json:"message" example:"invalid wildcard pattern"`
}

// HealthResponse is the response body for GET /api/v1/health.
type HealthResponse struct {
	Status            string `json:"status" example:"ok"`
	UptimeSeconds     int64  `json:"uptime_seconds" example:"3600"`
	ActiveConnections int64  `json:"active_connections" example:"5"`
	RuleCount         int    `json:"rule_count" example:"3"`
}

// RulesResponse is the response body for GET /api/v1/rules.
type RulesResponse struct {
	Rules map[string]RuleSetJSON `json:"rules"`
}

// RuleSetJSON represents a rule set in API responses.
type RuleSetJSON struct {
	Entries   []EntryJSON `json:"entries"`
	UpdatedAt time.Time   `json:"updated_at"`
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

// handleHealth godoc
//
//	@Summary		Health check
//	@Description	Returns the health status of the proxy and API server
//	@Tags			Health
//	@Produce		json
//	@Success		200	{object}	HealthResponse
//	@Router			/api/v1/health [get]
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := HealthResponse{
		Status:            "ok",
		UptimeSeconds:     int64(time.Since(h.startTime).Seconds()),
		ActiveConnections: h.proxyHandler.ActiveConnections(),
		RuleCount:         h.store.Count(),
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleGetRules godoc
//
//	@Summary		List all rules
//	@Description	Returns all registered whitelist rule sets keyed by source IP
//	@Tags			Rules
//	@Produce		json
//	@Success		200	{object}	RulesResponse
//	@Router			/api/v1/rules [get]
func (h *Handler) handleGetRules(w http.ResponseWriter, r *http.Request) {
	all := h.store.All()
	result := make(map[string]RuleSetJSON, len(all))
	for ip, rs := range all {
		entries := make([]EntryJSON, len(rs.Entries))
		for i, e := range rs.Entries {
			entries[i] = EntryJSON{Host: e.Host, Port: e.Port}
		}
		result[ip] = RuleSetJSON{Entries: entries, UpdatedAt: rs.UpdatedAt}
	}
	writeJSON(w, http.StatusOK, RulesResponse{Rules: result})
}

// handlePutRules godoc
//
//	@Summary		Set rules for a source IP
//	@Description	Replaces the entire rule set for the given source IP
//	@Tags			Rules
//	@Accept			json
//	@Produce		json
//	@Param			sourceIP	path		string			true	"Source IPv4 address"	example(172.20.0.3)
//	@Param			body		body		PutRulesRequest	true	"Rule entries"
//	@Success		200			{object}	PutRulesResponse
//	@Failure		400			{object}	ErrorResponse
//	@Router			/api/v1/rules/{sourceIP} [put]
func (h *Handler) handlePutRules(w http.ResponseWriter, r *http.Request) {
	sourceIP := r.PathValue("sourceIP")

	var req PutRulesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "failed to parse request body: " + err.Error(),
		})
		return
	}

	var details []ErrorDetail
	entries := make([]rule.Entry, len(req.Entries))
	for i, e := range req.Entries {
		entry := rule.Entry{Host: e.Host, Port: e.Port}
		if err := rule.ValidateEntry(entry); err != nil {
			details = append(details, ErrorDetail{
				Field:   fmt.Sprintf("entries[%d].host", i),
				Message: err.Error(),
			})
		}
		entries[i] = entry
	}

	if len(details) > 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: fmt.Sprintf("invalid host pattern: %s", req.Entries[0].Host),
			Details: details,
		})
		return
	}

	h.store.Set(sourceIP, entries)
	h.logger.Info("rules updated", "operation", "set", "src_ip", sourceIP, "entry_count", len(entries))

	rs, _ := h.store.Get(sourceIP)
	respEntries := make([]EntryJSON, len(rs.Entries))
	for i, e := range rs.Entries {
		respEntries[i] = EntryJSON{Host: e.Host, Port: e.Port}
	}
	writeJSON(w, http.StatusOK, PutRulesResponse{
		SourceIP:  sourceIP,
		Entries:   respEntries,
		UpdatedAt: rs.UpdatedAt,
	})
}

// handleDeleteRulesByIP godoc
//
//	@Summary		Delete rules for a source IP
//	@Description	Deletes the rule set for the given source IP
//	@Tags			Rules
//	@Produce		json
//	@Param			sourceIP	path	string	true	"Source IPv4 address"	example(172.20.0.3)
//	@Success		204			"Rules deleted"
//	@Failure		404			{object}	ErrorResponse
//	@Router			/api/v1/rules/{sourceIP} [delete]
func (h *Handler) handleDeleteRulesByIP(w http.ResponseWriter, r *http.Request) {
	sourceIP := r.PathValue("sourceIP")
	if !h.store.Delete(sourceIP) {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: fmt.Sprintf("no rules found for source IP: %s", sourceIP),
		})
		return
	}
	h.logger.Info("rules deleted", "operation", "delete", "src_ip", sourceIP)
	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteAllRules godoc
//
//	@Summary		Delete all rules
//	@Description	Deletes all rule sets for all source IPs
//	@Tags			Rules
//	@Success		204	"All rules deleted"
//	@Router			/api/v1/rules [delete]
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
