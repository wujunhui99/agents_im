package response

import (
	"encoding/json"
	"net/http"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/observability"
)

type Envelope struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func WriteOK(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, string(apperror.CodeOK), "ok", data)
}

func WriteError(w http.ResponseWriter, err error) {
	appErr := apperror.From(err)
	WriteJSON(w, apperror.HTTPStatus(err), string(appErr.Code), appErr.Message, nil)
}

func WriteJSON(w http.ResponseWriter, status int, code string, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

func GoZeroErrorHandler(err error) (int, any) {
	appErr := apperror.From(err)
	if appErr == nil {
		return http.StatusOK, Envelope{
			Code:    string(apperror.CodeOK),
			Message: "ok",
			Data:    nil,
		}
	}

	return apperror.HTTPStatus(err), Envelope{
		Code:    string(appErr.Code),
		Message: appErr.Message,
		Data:    nil,
	}
}

func GoZeroUnauthorizedCallback(w http.ResponseWriter, r *http.Request, _ error) {
	_, traceContext := observability.EnsureHTTPTrace(r)
	observability.InjectTraceHeaders(w, traceContext)
	WriteJSON(w, http.StatusUnauthorized, string(apperror.CodeUnauthenticated), "invalid or missing bearer token", nil)
}
