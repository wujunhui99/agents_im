package user

import (
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/objectstorage"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
)

type ServiceContext struct {
	common.AuthRuntime
	AccountLogic *logic.UserLogic
	UserLogic    *logic.UserLogic
	Repo         repository.Repository
	MediaLogic   *logic.MediaLogic
	MediaRepo    repository.MediaRepository
	ObjectStore  objectstorage.ObjectStore
	MessageRepo  repository.MessageRepository
}

func NewServiceContext(repo repository.Repository) *ServiceContext {
	return NewServiceContextWithAuth(repo, config.DefaultJWTAuthConfig())
}

func NewServiceContextWithAuth(repo repository.Repository, auth config.JWTAuthConfig) *ServiceContext {
	userLogic := logic.NewUserLogic(repo)
	return &ServiceContext{
		AuthRuntime:  common.NewAuthRuntime(auth),
		AccountLogic: userLogic,
		UserLogic:    userLogic,
		Repo:         repo,
	}
}

func NewServiceContextWithMedia(repo repository.Repository, mediaRepo repository.MediaRepository, objectStore objectstorage.ObjectStore, bucket string, auth config.JWTAuthConfig) *ServiceContext {
	ctx := NewServiceContextWithAuth(repo, auth)
	ctx.MediaRepo = mediaRepo
	ctx.ObjectStore = objectStore
	ctx.MediaLogic = logic.NewMediaLogic(mediaRepo, objectStore, bucket)
	return ctx
}

func (ctx *ServiceContext) ConfigureMediaAttachmentAccess(messageRepo repository.MessageRepository) {
	if ctx == nil || ctx.MediaLogic == nil || messageRepo == nil {
		return
	}
	ctx.MessageRepo = messageRepo
	ctx.MediaLogic.WithAttachmentAccessChecker(logic.NewMessageMediaAccessChecker(messageRepo))
}
