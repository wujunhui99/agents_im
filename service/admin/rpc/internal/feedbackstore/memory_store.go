package feedbackstore

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
)

// MemoryStore 是 Store 的进程内实现（单测/demo 用）。语义对齐 ModelStore：缺 feedback_id 时
// 自动分配、created_at desc 列表、Update 只改 status/admin_note。
type MemoryStore struct {
	mu     sync.RWMutex
	nextID uint64
	byID   map[string]model.Feedback
	now    func() time.Time
}

var _ Store = (*MemoryStore)(nil)

// NewMemoryStore 构建空的内存 Store。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		byID: make(map[string]model.Feedback),
		now:  time.Now,
	}
}

func (s *MemoryStore) CreateFeedback(_ context.Context, feedback model.Feedback) (model.Feedback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(feedback.FeedbackID) == "" {
		s.nextID++
		feedback.FeedbackID = fmt.Sprintf("fb_%06d", s.nextID)
	}
	if _, exists := s.byID[feedback.FeedbackID]; exists {
		return model.Feedback{}, apperror.AlreadyExists("feedback already exists")
	}
	if feedback.Status == "" {
		feedback.Status = model.FeedbackStatusNew
	}
	now := s.now().UTC()
	feedback.CreatedAt = now
	feedback.UpdatedAt = now
	s.byID[feedback.FeedbackID] = feedback.Clone()
	return feedback.Clone(), nil
}

func (s *MemoryStore) ListFeedback(_ context.Context, filter ListFilter) ([]model.Feedback, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]model.Feedback, 0, len(s.byID))
	for _, feedback := range s.byID {
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

func (s *MemoryStore) GetFeedback(_ context.Context, feedbackID string) (model.Feedback, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	feedback, ok := s.byID[strings.TrimSpace(feedbackID)]
	if !ok {
		return model.Feedback{}, apperror.NotFound("feedback not found")
	}
	return feedback.Clone(), nil
}

func (s *MemoryStore) UpdateFeedback(_ context.Context, feedback model.Feedback) (model.Feedback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.byID[feedback.FeedbackID]
	if !ok {
		return model.Feedback{}, apperror.NotFound("feedback not found")
	}
	current.Status = feedback.Status
	current.AdminNote = feedback.AdminNote
	current.UpdatedAt = s.now().UTC()
	s.byID[current.FeedbackID] = current.Clone()
	return current.Clone(), nil
}
