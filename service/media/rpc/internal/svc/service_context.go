package svc

import (
	"context"
	"log"

	sharedmodel "github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/objectstorage"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/model"
	"github.com/zeromicro/go-zero/core/stores/postgres"
)

// media_id 雪花中段（12 位）布局：media 无单/群语义，route hint 宽度为 0（保留，恒传 0，
// 见 routedflake.go 与 ADR #529）；低 10 位机器号（默认 1024 实例），高 2 位为保留间隙。
// 扩副本调 Snowflake.MachineBits（机器号靠右收缩、不挪位）。
const (
	mediaHintBits           = 0
	defaultMediaMachineBits = 10
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
	MediaIDGen       *idgen.RoutedFlake
	Store            objectstorage.ObjectStore
	Bucket           string
	Accounts         AccountReader
	AttachmentAccess AttachmentAccessChecker
}

func NewServiceContext(c config.Config) *ServiceContext {
	mediaModel := model.NewMediaObjectsModel(postgres.New(c.DataSource))

	mediaIDGen, err := newMediaIDGenerator(c.Snowflake)
	if err != nil {
		log.Fatalf("build media id generator: %v", err)
	}

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
		MediaIDGen:       mediaIDGen,
		Store:            objectStore,
		Bucket:           osCfg.Bucket,
		Accounts:         accountRepo,
		AttachmentAccess: newMessageAttachmentAccessChecker(messageRepo),
	}
}

// newMediaIDGenerator 构造 media_id 的 RoutedFlake。机器号优先用 idgen.ResolveMachineID()
// （env AGENTS_IM_SNOWFLAKE_MACHINE_ID 或 StatefulSet pod ordinal）；解析不到时回退到
// 配置值（默认 0，适用单副本）。多副本部署须经 env/ordinal 注入唯一机器号，否则同毫秒碰撞。
func newMediaIDGenerator(cfg config.SnowflakeConfig) (*idgen.RoutedFlake, error) {
	machineBits := cfg.MachineBits
	if machineBits == 0 {
		machineBits = defaultMediaMachineBits
	}
	machineID := cfg.MachineID
	if resolved, err := idgen.ResolveMachineID(); err == nil {
		machineID = resolved
	}
	return idgen.NewRoutedFlake(idgen.RoutedFlakeConfig{
		HintBits:    mediaHintBits,
		MachineBits: machineBits,
		MachineID:   machineID,
	})
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
