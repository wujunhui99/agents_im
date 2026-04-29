package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/wujunhui99/agents_im/internal/apperror"
	authlogic "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/internal/auth/svc"
	"github.com/wujunhui99/agents_im/internal/response"
)

func RegisterHandlers(mux *http.ServeMux, ctx *svc.ServiceContext) {
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/auth/register", registerHandler(ctx))
	mux.HandleFunc("/auth/login", loginHandler(ctx))
	mux.HandleFunc("/auth/validate", validateTokenHandler(ctx))
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	response.WriteOK(w, map[string]string{"status": "ok"})
}

func registerHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}

		var req authlogic.RegisterRequest
		if err := decodeJSON(r, &req); err != nil {
			response.WriteError(w, err)
			return
		}

		result, err := ctx.AuthLogic.Register(r.Context(), req)
		if err != nil {
			response.WriteError(w, err)
			return
		}
		response.WriteOK(w, result)
	}
}

func loginHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}

		var req authlogic.LoginRequest
		if err := decodeJSON(r, &req); err != nil {
			response.WriteError(w, err)
			return
		}

		result, err := ctx.AuthLogic.Login(r.Context(), req)
		if err != nil {
			response.WriteError(w, err)
			return
		}
		response.WriteOK(w, result)
	}
}

func validateTokenHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}

		var req authlogic.ValidateTokenRequest
		if err := decodeJSON(r, &req); err != nil {
			response.WriteError(w, err)
			return
		}

		result, err := ctx.AuthLogic.ValidateToken(r.Context(), req)
		if err != nil {
			response.WriteError(w, err)
			return
		}
		response.WriteOK(w, result)
	}
}

func decodeJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return apperror.InvalidArgument("invalid json body")
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return apperror.InvalidArgument("invalid json body")
	}
	return nil
}

func methodNotAllowed(w http.ResponseWriter) {
	response.WriteJSON(w, http.StatusMethodNotAllowed, string(apperror.CodeInvalidArgument), "method not allowed", nil)
}
