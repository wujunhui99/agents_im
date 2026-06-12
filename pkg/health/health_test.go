package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLivenessHandlerReportsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	LivenessHandler("message-api").ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var report Report
	if err := json.NewDecoder(rec.Body).Decode(&report); err != nil {
		t.Fatalf("decode liveness response: %v", err)
	}
	if report.Status != StatusOK || report.Service != "message-api" {
		t.Fatalf("unexpected liveness report: %+v", report)
	}
}

func TestReadinessHandlerReportsReady(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	ReadinessHandler("msggateway", func(*http.Request) []Check {
		return []Check{
			ComponentCheck("message_logic", true, "configured"),
			ComponentCheck("presence_store", true, "configured"),
		}
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var report Report
	if err := json.NewDecoder(rec.Body).Decode(&report); err != nil {
		t.Fatalf("decode readiness response: %v", err)
	}
	if report.Status != StatusReady || len(report.Checks) != 2 {
		t.Fatalf("unexpected readiness report: %+v", report)
	}
}

func TestReadinessHandlerReportsUnavailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	ReadinessHandler("msggateway", func(*http.Request) []Check {
		return []Check{
			ComponentCheck("message_logic", false, "not configured"),
		}
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var report Report
	if err := json.NewDecoder(rec.Body).Decode(&report); err != nil {
		t.Fatalf("decode readiness response: %v", err)
	}
	if report.Status != StatusNotReady || report.Checks[0].Status != StatusNotReady {
		t.Fatalf("unexpected readiness report: %+v", report)
	}
}
