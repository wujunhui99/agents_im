package repository

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/auth/model"
)

type CredentialRepository interface {
	Create(ctx context.Context, credential model.Credential) (model.Credential, error)
	GetByIdentifier(ctx context.Context, identifier string) (model.Credential, error)
}
