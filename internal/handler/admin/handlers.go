package admin

import (
	"net/http"

	business "github.com/wujunhui99/agents_im/internal/logic"
	adminsvc "github.com/wujunhui99/agents_im/internal/servicecontext/admin"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func DashboardHandler(svcCtx *adminsvc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminDashboardReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		resp, err := svcCtx.AdminLogic.GetDashboard(r.Context(), business.AdminDashboardRequest{Limit: int(req.Limit)})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, adminDashboardResp(resp))
	}
}

func ListLLMTracesHandler(svcCtx *adminsvc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminLLMTraceListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		resp, err := svcCtx.AdminLogic.ListLLMTraces(r.Context(), business.AdminLLMTraceListRequest{
			Status: req.Status,
			Limit:  int(req.Limit),
			Offset: int(req.Offset),
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, adminTraceListResp(resp))
	}
}

func GetLLMTraceHandler(svcCtx *adminsvc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminLLMTraceReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		resp, err := svcCtx.AdminLogic.GetLLMTraceDetail(r.Context(), business.AdminLLMTraceDetailRequest{TraceID: req.TraceID})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, adminTraceDetailResp(resp))
	}
}

func GetConversationMessagesHandler(svcCtx *adminsvc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminConversationMessagesReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		resp, err := svcCtx.AdminLogic.GetConversationMessages(r.Context(), business.AdminConversationMessagesRequest{
			ConversationID: req.ConversationID,
			FromSeq:        req.FromSeq,
			ToSeq:          req.ToSeq,
			Limit:          int(req.Limit),
			Order:          req.Order,
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, adminConversationMessagesResp(resp))
	}
}

func SearchUsersHandler(svcCtx *adminsvc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminUserSearchReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		resp, err := svcCtx.AdminLogic.SearchUsers(r.Context(), business.AdminUserSearchRequest{
			Query: req.Query,
			Limit: int(req.Limit),
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, adminUserSearchResp(resp))
	}
}

func GetUserHandler(svcCtx *adminsvc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminUserReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		resp, err := svcCtx.AdminLogic.GetUserDetail(r.Context(), business.AdminUserDetailRequest{AccountID: req.AccountID})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, adminUserDetailResp(resp))
	}
}

func GetUserFriendsHandler(svcCtx *adminsvc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminUserReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		resp, err := svcCtx.AdminLogic.GetUserFriends(r.Context(), business.AdminUserFriendsRequest{AccountID: req.AccountID})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, adminUserFriendsResp(resp))
	}
}

func GetUserConversationsHandler(svcCtx *adminsvc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminUserReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		resp, err := svcCtx.AdminLogic.GetUserConversations(r.Context(), business.AdminUserConversationsRequest{AccountID: req.AccountID})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, adminUserConversationsResp(resp))
	}
}

func ListFeedbackHandler(svcCtx *adminsvc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminFeedbackListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		resp, err := svcCtx.AdminLogic.ListFeedback(r.Context(), business.AdminFeedbackListRequest{
			Status: req.Status,
			Limit:  int(req.Limit),
			Offset: int(req.Offset),
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, adminFeedbackListResp(resp))
	}
}

func GetFeedbackHandler(svcCtx *adminsvc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminFeedbackReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		resp, err := svcCtx.AdminLogic.GetFeedback(r.Context(), business.AdminFeedbackDetailRequest{FeedbackID: req.FeedbackID})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, adminFeedbackDetailResp(resp))
	}
}

func UpdateFeedbackHandler(svcCtx *adminsvc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminFeedbackUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		resp, err := svcCtx.AdminLogic.UpdateFeedback(r.Context(), business.AdminFeedbackUpdateRequest{
			FeedbackID: req.FeedbackID,
			Status:     req.Status,
			AdminNote:  req.AdminNote,
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, adminFeedbackDetailResp(business.AdminFeedbackDetailResponse{Feedback: resp.Feedback}))
	}
}
