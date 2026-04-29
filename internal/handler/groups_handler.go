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

func RegisterGroupsHandlers(mux *http.ServeMux, ctx *svc.ServiceContext) {
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/groups", groupsHandler(ctx))
	mux.HandleFunc("/groups/", groupByIDHandler(ctx))
}

func groupsHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/groups" {
			response.WriteError(w, apperror.NotFound("route not found"))
			return
		}
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}

		userID, err := currentUserID(r)
		if err != nil {
			response.WriteError(w, err)
			return
		}

		var req createGroupRequest
		if err := decodeJSON(r, &req); err != nil {
			response.WriteError(w, err)
			return
		}

		group, err := ctx.GroupsLogic.CreateGroup(r.Context(), logic.CreateGroupRequest{
			CreatorUserID: userID,
			Name:          req.Name,
			Description:   req.Description,
		})
		if err != nil {
			response.WriteError(w, err)
			return
		}
		response.WriteOK(w, group)
	}
}

type createGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type addMemberRequest struct {
	UserID string `json:"user_id,omitempty"`
}

func groupByIDHandler(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts, err := splitGroupPath(r.URL.Path)
		if err != nil {
			response.WriteError(w, err)
			return
		}

		switch {
		case len(parts) == 1 && r.Method == http.MethodGet:
			getGroup(w, r, ctx, parts[0])
		case len(parts) == 2 && parts[1] == "members" && r.Method == http.MethodPost:
			addGroupMember(w, r, ctx, parts[0])
		case len(parts) == 2 && parts[1] == "members" && r.Method == http.MethodGet:
			listGroupMembers(w, r, ctx, parts[0])
		case len(parts) == 3 && parts[1] == "members" && parts[2] == "me" && r.Method == http.MethodDelete:
			leaveGroup(w, r, ctx, parts[0])
		case len(parts) == 1:
			methodNotAllowed(w)
		case len(parts) == 2 && parts[1] == "members":
			methodNotAllowed(w)
		case len(parts) == 3 && parts[1] == "members" && parts[2] == "me":
			methodNotAllowed(w)
		default:
			response.WriteError(w, apperror.NotFound("route not found"))
		}
	}
}

func getGroup(w http.ResponseWriter, r *http.Request, ctx *svc.ServiceContext, groupID string) {
	group, err := ctx.GroupsLogic.GetGroup(r.Context(), logic.GetGroupRequest{GroupID: groupID})
	if err != nil {
		response.WriteError(w, err)
		return
	}
	response.WriteOK(w, group)
}

func addGroupMember(w http.ResponseWriter, r *http.Request, ctx *svc.ServiceContext, groupID string) {
	userID, err := currentUserID(r)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	var req addMemberRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		response.WriteError(w, err)
		return
	}

	member, err := ctx.GroupsLogic.AddMember(r.Context(), logic.AddMemberRequest{
		GroupID:        groupID,
		OperatorUserID: userID,
		UserID:         req.UserID,
	})
	if err != nil {
		response.WriteError(w, err)
		return
	}
	response.WriteOK(w, member)
}

func leaveGroup(w http.ResponseWriter, r *http.Request, ctx *svc.ServiceContext, groupID string) {
	userID, err := currentUserID(r)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	member, err := ctx.GroupsLogic.LeaveGroup(r.Context(), logic.LeaveGroupRequest{
		GroupID: groupID,
		UserID:  userID,
	})
	if err != nil {
		response.WriteError(w, err)
		return
	}
	response.WriteOK(w, member)
}

func listGroupMembers(w http.ResponseWriter, r *http.Request, ctx *svc.ServiceContext, groupID string) {
	members, err := ctx.GroupsLogic.ListMembers(r.Context(), logic.ListMembersRequest{GroupID: groupID})
	if err != nil {
		response.WriteError(w, err)
		return
	}
	response.WriteOK(w, members)
}

func splitGroupPath(path string) ([]string, error) {
	rest := strings.TrimPrefix(path, "/groups/")
	if rest == "" || rest == path {
		return nil, apperror.NotFound("route not found")
	}

	rawParts := strings.Split(rest, "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if part == "" {
			return nil, apperror.NotFound("route not found")
		}
		decoded, err := url.PathUnescape(part)
		if err != nil {
			return nil, apperror.InvalidArgument("path is invalid")
		}
		parts = append(parts, decoded)
	}

	return parts, nil
}

func decodeOptionalJSON(r *http.Request, dst interface{}) error {
	if r.Body == nil || r.Body == http.NoBody {
		return nil
	}
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		if err == io.EOF {
			return nil
		}
		return apperror.InvalidArgument("invalid json body")
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return apperror.InvalidArgument("invalid json body")
	}
	return nil
}
