package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/response"
	"github.com/wujunhui99/agents_im/internal/svc"
)

func RegisterHandlers(mux *http.ServeMux, ctx *svc.ServiceContext) {
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/me", meHandler(ctx))
	mux.HandleFunc("/users", usersHandler(ctx))
	mux.HandleFunc("/users/exists", existsHandler(ctx))
	mux.HandleFunc("/users/", userByIdentifierHandler(ctx))
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	response.WriteOK(w, map[string]string{"status": "ok"})
}

func meHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			userID, err := currentUserID(r)
			if err != nil {
				response.WriteError(w, err)
				return
			}

			user, err := ctx.UserLogic.GetUserByID(r.Context(), logic.GetUserByIDRequest{UserID: userID})
			if err != nil {
				response.WriteError(w, err)
				return
			}
			response.WriteOK(w, user)
		case http.MethodPatch:
			userID, err := currentUserID(r)
			if err != nil {
				response.WriteError(w, err)
				return
			}

			var req updateMeRequest
			if err := decodeJSON(r, &req); err != nil {
				response.WriteError(w, err)
				return
			}

			user, err := ctx.UserLogic.UpdateUserProfile(r.Context(), logic.UpdateUserProfileRequest{
				UserID:      userID,
				DisplayName: req.DisplayName,
				Name:        req.Name,
				Gender:      req.Gender,
				Age:         req.Age,
				Region:      req.Region,
			})
			if err != nil {
				response.WriteError(w, err)
				return
			}
			response.WriteOK(w, user)
		default:
			methodNotAllowed(w)
		}
	}
}

type updateMeRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	Name        *string `json:"name,omitempty"`
	Gender      *string `json:"gender,omitempty"`
	Age         *int32  `json:"age,omitempty"`
	Region      *string `json:"region,omitempty"`
}

func usersHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users" {
			response.WriteError(w, apperror.NotFound("route not found"))
			return
		}
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}

		var req logic.CreateUserRequest
		if err := decodeJSON(r, &req); err != nil {
			response.WriteError(w, err)
			return
		}

		user, err := ctx.UserLogic.CreateUser(r.Context(), req)
		if err != nil {
			response.WriteError(w, err)
			return
		}
		response.WriteOK(w, user)
	}
}

func existsHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}

		result, err := ctx.UserLogic.ExistsByIdentifier(r.Context(), logic.ExistsByIdentifierRequest{
			Identifier: r.URL.Query().Get("identifier"),
		})
		if err != nil {
			response.WriteError(w, err)
			return
		}
		response.WriteOK(w, result)
	}
}

func userByIdentifierHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}

		identifier := strings.TrimPrefix(r.URL.Path, "/users/")
		if identifier == "" || strings.Contains(identifier, "/") {
			response.WriteError(w, apperror.NotFound("route not found"))
			return
		}
		identifier, err := url.PathUnescape(identifier)
		if err != nil {
			response.WriteError(w, apperror.InvalidArgument("identifier path is invalid"))
			return
		}

		user, err := ctx.UserLogic.GetUserByIdentifier(r.Context(), logic.GetUserByIdentifierRequest{
			Identifier: identifier,
		})
		if err != nil {
			response.WriteError(w, err)
			return
		}
		response.WriteOK(w, user)
	}
}

func currentUserID(r *http.Request) (string, error) {
	userID := strings.TrimSpace(r.Header.Get("X-User-Id"))
	if userID == "" {
		return "", apperror.Unauthenticated("X-User-Id header is required")
	}
	return userID, nil
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
