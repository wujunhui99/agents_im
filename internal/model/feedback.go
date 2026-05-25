package model

import "time"

type FeedbackCategory string

const (
	FeedbackCategoryBug            FeedbackCategory = "bug"
	FeedbackCategoryPoorExperience FeedbackCategory = "poor_experience"
	FeedbackCategoryFeatureRequest FeedbackCategory = "feature_request"
	FeedbackCategoryOther          FeedbackCategory = "other"
)

type FeedbackStatus string

const (
	FeedbackStatusNew      FeedbackStatus = "new"
	FeedbackStatusTriaged  FeedbackStatus = "triaged"
	FeedbackStatusPlanned  FeedbackStatus = "planned"
	FeedbackStatusResolved FeedbackStatus = "resolved"
	FeedbackStatusRejected FeedbackStatus = "rejected"
)

type Feedback struct {
	FeedbackID string
	UserID     string
	Category   FeedbackCategory
	Status     FeedbackStatus
	Title      string
	Content    string
	Contact    string
	PageURL    string
	UserAgent  string
	ClientMeta map[string]any
	AdminNote  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NormalizeFeedbackCategory(value string) (FeedbackCategory, bool) {
	switch FeedbackCategory(value) {
	case FeedbackCategoryBug, FeedbackCategoryPoorExperience, FeedbackCategoryFeatureRequest, FeedbackCategoryOther:
		return FeedbackCategory(value), true
	default:
		return "", false
	}
}

func NormalizeFeedbackStatus(value string) (FeedbackStatus, bool) {
	switch FeedbackStatus(value) {
	case FeedbackStatusNew, FeedbackStatusTriaged, FeedbackStatusPlanned, FeedbackStatusResolved, FeedbackStatusRejected:
		return FeedbackStatus(value), true
	default:
		return "", false
	}
}

func (f Feedback) Clone() Feedback {
	if f.ClientMeta != nil {
		cloneMeta := make(map[string]any, len(f.ClientMeta))
		for key, value := range f.ClientMeta {
			cloneMeta[key] = value
		}
		f.ClientMeta = cloneMeta
	}
	return f
}
