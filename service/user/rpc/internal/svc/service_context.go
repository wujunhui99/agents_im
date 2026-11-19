package svc

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/resource/rpc/mediaclient"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/zrpc"
)

// UserEventPublisher 发布 user.created.v1；agent-rpc 消费后完成默认助手绑定与好友建立。
// 这是 user-rpc 唯一对外的跨域开通信号（不再同步调用 agent-rpc/friends-rpc）。
type UserEventPublisher interface {
	PublishUserCreated(ctx context.Context, accountID, accountType string) error
}

// kafkaUserEventPublisher 是 UserEventPublisher 的 Kafka 实现。
type kafkaUserEventPublisher struct {
	producer *messaging.KafkaProducer
}

func (p *kafkaUserEventPublisher) PublishUserCreated(ctx context.Context, accountID, accountType string) error {
	payload, err := messaging.MarshalUserCreatedEvent(messaging.UserCreatedEvent{
		EventID:     uuid.NewString(),
		AccountID:   accountID,
		AccountType: accountType,
		CreatedAt:   time.Now().Unix(),
	})
	if err != nil {
		return err
	}
	return p.producer.Publish(ctx, messaging.TopicUserCreated, accountID, payload)
}

// AvatarValidator 校验头像 media 的存在/归属/类型（media 域）。#533 起经 media-rpc，
// 不再直读 media_objects（脱 internal/mediavalidate）。
type AvatarValidator interface {
	ValidateAvatarMedia(ctx context.Context, ownerUserID string, mediaID string) error
}

type ServiceContext struct {
	Config   config.Config
	Accounts model.AccountsModel
	Profiles model.ProfilesModel

	Events          UserEventPublisher
	AvatarValidator AvatarValidator
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := postgres.New(c.DataSource)
	accountsModel := model.NewAccountsModel(conn)
	profilesModel := model.NewProfilesModel(conn)

	// 默认助手「账号」由 user 域本地幂等 ensure；agent 域装配 + 好友关系由 agent-rpc
	// 消费 user.created.v1 完成（user-rpc 不再调用 agent-rpc/friends-rpc）。
	accountRepo := newAssistantAccountRepo(accountsModel, profilesModel)
	if _, err := newAssistantAccountEnsurer(accountRepo).Ensure(context.Background()); err != nil {
		log.Fatalf("ensure default assistant account: %v", err)
	}

	// 事件出口：user.created.v1（Kafka 必填，与 msg-rpc 写链路一致）。
	brokers := appconfig.KafkaBrokerList(firstNonEmpty(strings.TrimSpace(os.ExpandEnv(c.Kafka.Brokers)), os.Getenv("KAFKA_BROKERS")))
	if len(brokers) == 0 {
		log.Fatalf("user-rpc requires kafka brokers to publish %s (set KAFKA_BROKERS)", messaging.TopicUserCreated)
	}
	producer, err := messaging.NewKafkaProducer(brokers)
	if err != nil {
		log.Fatalf("build kafka producer: %v", err)
	}
	logx.Infof("user-rpc publishes %s via %v", messaging.TopicUserCreated, brokers)

	// 头像 media 校验经 media-rpc（#533，脱 internal/mediavalidate 直读 media_objects）。
	mediaCli, err := zrpc.NewClient(c.ResourceRPC)
	if err != nil {
		log.Fatalf("build media rpc client: %v", err)
	}

	return &ServiceContext{
		Config:          c,
		Accounts:        accountsModel,
		Profiles:        profilesModel,
		Events:          &kafkaUserEventPublisher{producer: producer},
		AvatarValidator: newResourceRPCAvatarValidator(mediaclient.NewMedia(mediaCli)),
	}
}

// hasRPCClientConfig 判断 zrpc 客户端是否已配置（target / endpoints / etcd 任一）。
func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
