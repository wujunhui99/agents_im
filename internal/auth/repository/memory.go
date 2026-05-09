package repository

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/auth/model"
)

type MemoryRepository struct {
	mu                   sync.RWMutex
	byIdentifier         map[string]model.Credential
	byUserID             map[string]string
	byEmail              map[string]string
	sessions             map[string]model.ActiveSession
	emailVerifications   map[string]model.EmailVerificationToken
	emailVerificationIDs []string
	now                  func() time.Time
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		byIdentifier:       make(map[string]model.Credential),
		byUserID:           make(map[string]string),
		byEmail:            make(map[string]string),
		sessions:           make(map[string]model.ActiveSession),
		emailVerifications: make(map[string]model.EmailVerificationToken),
		now:                time.Now,
	}
}

func (r *MemoryRepository) Create(_ context.Context, credential model.Credential) (model.Credential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byIdentifier[credential.Identifier]; exists {
		return model.Credential{}, apperror.AlreadyExists("auth credential already exists")
	}
	if _, exists := r.byUserID[credential.UserID]; exists {
		return model.Credential{}, apperror.AlreadyExists("auth credential already exists")
	}
	if credential.Email != "" {
		if _, exists := r.byEmail[credential.Email]; exists {
			return model.Credential{}, apperror.AlreadyExists("auth email already exists")
		}
	}

	now := r.now().UTC()
	credential.CreatedAt = now
	credential.UpdatedAt = now

	r.byIdentifier[credential.Identifier] = credential.Clone()
	r.byUserID[credential.UserID] = credential.Identifier
	if credential.Email != "" {
		r.byEmail[credential.Email] = credential.Identifier
	}
	return credential.Clone(), nil
}

func (r *MemoryRepository) SetActiveSession(_ context.Context, session model.ActiveSession) error {
	session.UserID = strings.TrimSpace(session.UserID)
	session.SessionID = strings.TrimSpace(session.SessionID)
	if session.UserID == "" || session.SessionID == "" {
		return apperror.InvalidArgument("active session requires user_id and session_id")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byUserID[session.UserID]; !exists {
		return apperror.NotFound("auth credential not found")
	}
	now := r.now().UTC()
	session.IssuedAt = session.IssuedAt.UTC()
	session.ExpiresAt = session.ExpiresAt.UTC()
	session.UpdatedAt = now
	r.sessions[session.UserID] = session.Clone()
	return nil
}

func (r *MemoryRepository) GetActiveSession(_ context.Context, userID string) (model.ActiveSession, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return model.ActiveSession{}, apperror.InvalidArgument("user_id is required")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	session, exists := r.sessions[userID]
	if !exists {
		return model.ActiveSession{}, apperror.NotFound("active session not found")
	}
	return session.Clone(), nil
}

func (r *MemoryRepository) GetByIdentifier(_ context.Context, identifier string) (model.Credential, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	credential, exists := r.byIdentifier[identifier]
	if !exists {
		return model.Credential{}, apperror.NotFound("auth credential not found")
	}

	return credential.Clone(), nil
}

func (r *MemoryRepository) GetByEmail(_ context.Context, email string) (model.Credential, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	identifier, exists := r.byEmail[email]
	if !exists {
		return model.Credential{}, apperror.NotFound("auth credential not found")
	}
	credential, exists := r.byIdentifier[identifier]
	if !exists {
		return model.Credential{}, apperror.NotFound("auth credential not found")
	}

	return credential.Clone(), nil
}

func (r *MemoryRepository) CreateEmailVerification(_ context.Context, token model.EmailVerificationToken) (model.EmailVerificationToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if strings.TrimSpace(token.ID) == "" {
		return model.EmailVerificationToken{}, apperror.InvalidArgument("email verification token id is required")
	}
	if strings.TrimSpace(token.Purpose) == "" || strings.TrimSpace(token.Email) == "" {
		return model.EmailVerificationToken{}, apperror.InvalidArgument("email verification token requires purpose and email")
	}
	if _, exists := r.emailVerifications[token.ID]; exists {
		return model.EmailVerificationToken{}, apperror.AlreadyExists("email verification token already exists")
	}

	now := r.now().UTC()
	for id, existing := range r.emailVerifications {
		if existing.Purpose == token.Purpose && existing.Email == token.Email && existing.ConsumedAt.IsZero() {
			existing.ConsumedAt = now
			existing.UpdatedAt = now
			r.emailVerifications[id] = existing
		}
	}

	token.CreatedAt = now
	token.UpdatedAt = now
	if token.LastSentAt.IsZero() {
		token.LastSentAt = now
	}
	r.emailVerifications[token.ID] = token.Clone()
	r.emailVerificationIDs = append(r.emailVerificationIDs, token.ID)
	return token.Clone(), nil
}

func (r *MemoryRepository) LatestEmailVerification(_ context.Context, purpose string, email string) (model.EmailVerificationToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := len(r.emailVerificationIDs) - 1; i >= 0; i-- {
		token := r.emailVerifications[r.emailVerificationIDs[i]]
		if token.Purpose == purpose && token.Email == email {
			return token.Clone(), nil
		}
	}
	return model.EmailVerificationToken{}, apperror.NotFound("email verification token not found")
}

func (r *MemoryRepository) IncrementEmailVerificationAttempts(_ context.Context, id string, now time.Time) (model.EmailVerificationToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	token, exists := r.emailVerifications[id]
	if !exists {
		return model.EmailVerificationToken{}, apperror.NotFound("email verification token not found")
	}
	token.AttemptCount++
	token.UpdatedAt = now.UTC()
	r.emailVerifications[id] = token
	return token.Clone(), nil
}

func (r *MemoryRepository) ConsumeEmailVerification(_ context.Context, id string, now time.Time) (model.EmailVerificationToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	token, exists := r.emailVerifications[id]
	if !exists {
		return model.EmailVerificationToken{}, apperror.NotFound("email verification token not found")
	}
	if !token.ConsumedAt.IsZero() {
		return model.EmailVerificationToken{}, apperror.InvalidArgument("email verification code is invalid or expired")
	}
	now = now.UTC()
	if !token.ExpiresAt.IsZero() && !now.Before(token.ExpiresAt) {
		return model.EmailVerificationToken{}, apperror.InvalidArgument("email verification code is invalid or expired")
	}
	token.AttemptCount++
	token.ConsumedAt = now
	token.UpdatedAt = now
	r.emailVerifications[id] = token
	return token.Clone(), nil
}
