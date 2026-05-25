package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
)

type FeedbackListFilter struct {
	Status model.FeedbackStatus
	Limit  int
	Offset int
}

type FeedbackRepository interface {
	CreateFeedback(ctx context.Context, feedback model.Feedback) (model.Feedback, error)
	ListFeedback(ctx context.Context, filter FeedbackListFilter) ([]model.Feedback, error)
	GetFeedback(ctx context.Context, feedbackID string) (model.Feedback, error)
	UpdateFeedback(ctx context.Context, feedback model.Feedback) (model.Feedback, error)
}

type MemoryFeedbackRepository struct {
	mu     sync.RWMutex
	nextID uint64
	byID   map[string]model.Feedback
	now    func() time.Time
}

func NewMemoryFeedbackRepository() *MemoryFeedbackRepository {
	return &MemoryFeedbackRepository{
		byID: make(map[string]model.Feedback),
		now:  time.Now,
	}
}

func (r *MemoryFeedbackRepository) CreateFeedback(_ context.Context, feedback model.Feedback) (model.Feedback, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.TrimSpace(feedback.FeedbackID) == "" {
		r.nextID++
		feedback.FeedbackID = fmt.Sprintf("fb_%06d", r.nextID)
	}
	if _, exists := r.byID[feedback.FeedbackID]; exists {
		return model.Feedback{}, apperror.AlreadyExists("feedback already exists")
	}
	now := r.now().UTC()
	feedback.CreatedAt = now
	feedback.UpdatedAt = now
	r.byID[feedback.FeedbackID] = feedback.Clone()
	return feedback.Clone(), nil
}

func (r *MemoryFeedbackRepository) ListFeedback(_ context.Context, filter FeedbackListFilter) ([]model.Feedback, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]model.Feedback, 0, len(r.byID))
	for _, feedback := range r.byID {
		if filter.Status != "" && feedback.Status != filter.Status {
			continue
		}
		items = append(items, feedback.Clone())
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].FeedbackID > items[j].FeedbackID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []model.Feedback{}, nil
	}
	items = items[offset:]
	if filter.Limit > 0 && filter.Limit < len(items) {
		items = items[:filter.Limit]
	}
	return items, nil
}

func (r *MemoryFeedbackRepository) GetFeedback(_ context.Context, feedbackID string) (model.Feedback, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	feedback, ok := r.byID[feedbackID]
	if !ok {
		return model.Feedback{}, apperror.NotFound("feedback not found")
	}
	return feedback.Clone(), nil
}

func (r *MemoryFeedbackRepository) UpdateFeedback(_ context.Context, feedback model.Feedback) (model.Feedback, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.byID[feedback.FeedbackID]
	if !ok {
		return model.Feedback{}, apperror.NotFound("feedback not found")
	}
	current.Status = feedback.Status
	current.AdminNote = feedback.AdminNote
	current.UpdatedAt = r.now().UTC()
	r.byID[current.FeedbackID] = current.Clone()
	return current.Clone(), nil
}
