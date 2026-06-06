package repository

import (
	"context"
	"time"

	"github.com/wujunhui99/agents_im/internal/auth/model"
)

type CredentialRepository interface {
	ActiveSessionRepository
	Create(ctx context.Context, credential model.Credential) (model.Credential, error)
	GetByIdentifier(ctx context.Context, identifier string) (model.Credential, error)
	GetByEmail(ctx context.Context, email string) (model.Credential, error)
}

type Repository interface {
	CredentialRepository
	EmailVerificationRepository
}

type ActiveSessionRepository interface {
	SetActiveSession(ctx context.Context, session model.ActiveSession) error
	GetActiveSession(ctx context.Context, userID string) (model.ActiveSession, error)
}

type EmailVerificationRepository interface {
	CreateEmailVerification(ctx context.Context, token model.EmailVerificationToken) (model.EmailVerificationToken, error)
	LatestEmailVerification(ctx context.Context, purpose string, email string) (model.EmailVerificationToken, error)
	IncrementEmailVerificationAttempts(ctx context.Context, id string, now time.Time) (model.EmailVerificationToken, error)
	ConsumeEmailVerification(ctx context.Context, id string, now time.Time) (model.EmailVerificationToken, error)
}
