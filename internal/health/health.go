package health

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const (
	StatusOK       = "ok"
	StatusReady    = "ready"
	StatusNotReady = "not_ready"
)

type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Report struct {
	Status    string  `json:"status"`
	Service   string  `json:"service"`
	Timestamp string  `json:"timestamp"`
	Checks    []Check `json:"checks,omitempty"`
}

func Liveness(service string, now time.Time) Report {
	return Report{
		Status:    StatusOK,
		Service:   normalizeService(service),
		Timestamp: now.UTC().Format(time.RFC3339),
	}
}

func Readiness(service string, now time.Time, checks []Check) Report {
	status := StatusReady
	normalized := make([]Check, 0, len(checks))
	for _, check := range checks {
		check.Name = strings.TrimSpace(check.Name)
		if check.Name == "" {
			check.Name = "unnamed"
		}
		if check.Status != StatusReady {
			check.Status = StatusNotReady
			status = StatusNotReady
		}
		normalized = append(normalized, check)
	}

	return Report{
		Status:    status,
		Service:   normalizeService(service),
		Timestamp: now.UTC().Format(time.RFC3339),
		Checks:    normalized,
	}
}

func ComponentCheck(name string, ready bool, message string) Check {
	status := StatusReady
	if !ready {
		status = StatusNotReady
	}
	return Check{
		Name:    strings.TrimSpace(name),
		Status:  status,
		Message: strings.TrimSpace(message),
	}
}

func LivenessHandler(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		WriteReport(w, http.StatusOK, Liveness(service, time.Now()))
	}
}

func ReadinessHandler(service string, checks func(*http.Request) []Check) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var current []Check
		if checks != nil {
			current = checks(r)
		}
		report := Readiness(service, time.Now(), current)
		status := http.StatusOK
		if report.Status != StatusReady {
			status = http.StatusServiceUnavailable
		}
		WriteReport(w, status, report)
	}
}

func WriteReport(w http.ResponseWriter, status int, report Report) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(report)
}

func normalizeService(service string) string {
	if service = strings.TrimSpace(service); service != "" {
		return service
	}
	return "agents-im"
}
