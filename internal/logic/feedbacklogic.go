package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type FeedbackLogic struct {
	repo repository.FeedbackRepository
}

type CreateFeedbackRequest struct {
	UserID     string
	Category   string
	Title      string
	Content    string
	Contact    string
	PageURL    string
	UserAgent  string
	ClientMeta map[string]any
}

func NewFeedbackLogic(repo repository.FeedbackRepository) *FeedbackLogic {
	return &FeedbackLogic{repo: repo}
}

func (l *FeedbackLogic) CreateFeedback(ctx context.Context, req CreateFeedbackRequest) (model.Feedback, error) {
	if l == nil || l.repo == nil {
		return model.Feedback{}, apperror.Internal("feedback repository is not configured")
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return model.Feedback{}, apperror.InvalidArgument("user_id is required")
	}
	category, ok := model.NormalizeFeedbackCategory(strings.TrimSpace(req.Category))
	if !ok {
		return model.Feedback{}, apperror.InvalidArgument("feedback category is invalid")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return model.Feedback{}, apperror.InvalidArgument("feedback title is required")
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return model.Feedback{}, apperror.InvalidArgument("feedback content is required")
	}
	return l.repo.CreateFeedback(ctx, model.Feedback{
		UserID:     userID,
		Category:   category,
		Status:     model.FeedbackStatusNew,
		Title:      title,
		Content:    content,
		Contact:    strings.TrimSpace(req.Contact),
		PageURL:    strings.TrimSpace(req.PageURL),
		UserAgent:  strings.TrimSpace(req.UserAgent),
		ClientMeta: req.ClientMeta,
	})
}
