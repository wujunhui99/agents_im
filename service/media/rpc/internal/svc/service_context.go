package svc

import (
	"log"

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

// ServiceContext 持有 media-rpc 的数据层与对象存储。media_objects 写入/读取走 goctl MediaModel
// （脱 internal/repository）。下载授权（EPIC #527 §4）的跨域编排在 media-api(BFF) 完成，故
// media-rpc 不持有任何跨域 rpc 客户端、保持叶子（无 rpc→rpc 调用）。
type ServiceContext struct {
	Config     config.Config
	MediaModel model.MediaObjectsModel
	MediaIDGen *idgen.RoutedFlake
	Store      objectstorage.ObjectStore
	Bucket     string
}

func NewServiceContext(c config.Config) *ServiceContext {
	mediaModel := model.NewMediaObjectsModel(postgres.New(c.DataSource))

	mediaIDGen, err := newMediaIDGenerator(c.Snowflake)
	if err != nil {
		log.Fatalf("build media id generator: %v", err)
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
		Config:     c,
		MediaModel: mediaModel,
		MediaIDGen: mediaIDGen,
		Store:      objectStore,
		Bucket:     osCfg.Bucket,
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
