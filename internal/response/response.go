package response

import (
	"encoding/json"
	"net/http"

	"github.com/wujunhui99/agents_im/internal/apperror"
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
