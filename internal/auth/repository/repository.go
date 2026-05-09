package repository

import (
	"context"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/auth/model"
	"github.com/wujunhui99/agents_im/internal/auth/token"
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

func ValidateActiveSession(ctx context.Context, repo ActiveSessionRepository, claims token.Claims) error {
	if repo == nil {
		return apperror.Internal("active session repository is required")
	}
	userID := strings.TrimSpace(claims.UserID)
	sessionID := strings.TrimSpace(claims.SessionID)
	if userID == "" || sessionID == "" {
		return apperror.Unauthenticated("token session is not active")
	}

	active, err := repo.GetActiveSession(ctx, userID)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return apperror.Unauthenticated("token session is not active")
		}
		return err
	}
	if strings.TrimSpace(active.SessionID) != sessionID {
		return apperror.Unauthenticated("token session is not active")
	}
	return nil
}
