package svc

import (
	"context"
	"log"

	sharedmodel "github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/objectstorage"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/model"
	"github.com/zeromicro/go-zero/core/stores/postgres"
)

// AccountReader 读用户账号（下载鉴权的管理员判定）。由 internal/repository 满足——这是 keystone 阻塞的
// 跨域读：暂无 user-rpc 接口可 BFF 化（见 issue #433 保留项），待 message-rpc 落地后改 BFF。
type AccountReader interface {
	GetByID(ctx context.Context, accountID string) (sharedmodel.User, error)
}

// AttachmentAccessChecker 判定请求者能否访问某消息附件媒体（下载鉴权的附件可见性）。由 internal/repository
// 的 message repo 满足，同属 keystone 阻塞的跨域读。
type AttachmentAccessChecker interface {
	UserCanAccessMedia(ctx context.Context, userID string, mediaID string) (bool, error)
}

// ServiceContext 持有 media-rpc 的数据层与对象存储。media_objects 写入/读取走 goctl MediaModel（脱
// internal/repository）；下载鉴权的跨域读（Accounts/AttachmentAccess）仍读 internal/repository，待 BFF 化。
type ServiceContext struct {
	Config           config.Config
	MediaModel       model.MediaObjectsModel
	Store            objectstorage.ObjectStore
	Bucket           string
	Accounts         AccountReader
	AttachmentAccess AttachmentAccessChecker
}

func NewServiceContext(c config.Config) *ServiceContext {
	mediaModel := model.NewMediaObjectsModel(postgres.New(c.DataSource))

	accountRepo, err := repository.NewRepositoryForStorage("postgres", c.DataSource)
	if err != nil {
		log.Fatalf("build account repository: %v", err)
	}
	messageRepo, err := repository.NewMessageRepositoryForStorage("postgres", c.DataSource)
	if err != nil {
		log.Fatalf("build message repository: %v", err)
	}

	osCfg, err := appconfig.ResolveObjectStorageConfig(c.ObjectStorage, "postgres")
	if err != nil {
		log.Fatalf("resolve object storage config: %v", err)
	}
	objectStore, err := objectstorage.NewStore(osCfg)
	if err != nil {
		log.Fatalf("build object storage: %v", err)
	}

	return &ServiceContext{
		Config:           c,
		MediaModel:       mediaModel,
		Store:            objectStore,
		Bucket:           osCfg.Bucket,
		Accounts:         accountRepo,
		AttachmentAccess: newMessageAttachmentAccessChecker(messageRepo),
	}
}

// messageAttachmentAccessChecker 把 message repo 的附件可见性查询适配成 AttachmentAccessChecker。
type messageAttachmentAccessChecker struct {
	repo repository.MessageRepository
}

func newMessageAttachmentAccessChecker(repo repository.MessageRepository) AttachmentAccessChecker {
	return messageAttachmentAccessChecker{repo: repo}
}

func (c messageAttachmentAccessChecker) UserCanAccessMedia(ctx context.Context, userID string, mediaID string) (bool, error) {
	if c.repo == nil {
		return false, nil
	}
	return c.repo.UserCanAccessMedia(ctx, userID, mediaID)
}
