package handler

import (
	"net/http"

	"github.com/unowned-22/api/internal/domain/health"
	"github.com/unowned-22/api/internal/transport/http/response"
)

type HealthHandler struct {
	checker health.Checker
}

func NewHealthHandler(checker health.Checker) *HealthHandler {
	return &HealthHandler{checker: checker}
}

func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	response.SendSuccess(w, http.StatusOK, health.Report{
		Status: health.StatusOK,
		Checks: make(map[string]health.CheckResult),
	})
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	report := h.checker.Check(r.Context())

	status := http.StatusOK
	if report.Status == health.StatusFail {
		status = http.StatusServiceUnavailable
	}

	response.SendSuccess(w, status, report)
}
