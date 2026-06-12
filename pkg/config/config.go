package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/observability"
	gozerotrace "github.com/zeromicro/go-zero/core/trace"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
	"gopkg.in/yaml.v3"
)

type APIConfig struct {
	Name             string
	Host             string
	Port             int
	Auth             JWTAuthConfig
	AdminBootstrap   AdminBootstrapConfig
	StorageDriver    string
	DataSource       string
	Redis            RedisConfig
	Presence         PresenceConfig
	Tracing          observability.TracingConfig
	DeepSeek         DeepSeekConfig
	LLMObservability LLMObservabilityConfig
	GatewayWS        GatewayWSConfig
	ObjectStorage    ObjectStorageConfig
	PythonExecutor   PythonExecutorConfig
	MailRPC          zrpc.RpcClientConf
	// MsgRPC 是 msggateway → msg-rpc 的 gRPC 客户端配置（03 §9 A3）。
	MsgRPC zrpc.RpcClientConf
}

type AdminBootstrapConfig struct {
	Identifier  string
	Password    string
	DisplayName string
}

type RPCConfig struct {
	Name          string
	ListenOn      string
	Auth          JWTAuthConfig
	StorageDriver string
	DataSource    string
	Redis         RedisConfig
	Presence      PresenceConfig
	Tracing       observability.TracingConfig
	MailRPC       zrpc.RpcClientConf
}

type JWTAuthConfig struct {
	AccessSecret string
	AccessExpire int64
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type PresenceConfig struct {
	Driver              string
	HeartbeatTTLSeconds int64
	KeyPrefix           string
}

type GatewayWSConfig struct {
	AllowedOrigins            []string
	AllowQueryToken           bool
	PingIntervalSeconds       int64
	HeartbeatTimeoutSeconds   int64
	CommandRateLimitPerSecond int
	CommandRateLimitBurst     int
}

type DeepSeekConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

type LLMObservabilityConfig struct {
	Enabled        bool
	Backend        string
	CaptureOutput  bool
	MaxOutputBytes int
	Langfuse       LangfuseObservabilityConfig
}

type LangfuseObservabilityConfig struct {
	Host      string
	PublicKey string
	SecretKey string
}

type ObjectStorageConfig struct {
	Driver           string
	Endpoint         string
	ExternalEndpoint string
	Bucket           string
	Region           string
	UseSSL           bool
	ExternalUseSSL   *bool
	AccessKeyID      string
	SecretAccessKey  string
}

type PythonExecutorConfig struct {
	Backend               string
	DefaultTimeoutSeconds int
	MaxTimeoutSeconds     int
	DefaultMemoryMiB      int
	MaxMemoryMiB          int
	MaxOutputBytes        int64
	K8S                   PythonExecutorK8SConfig
}

type PythonExecutorK8SConfig struct {
	Namespace          string
	Image              string
	ServiceAccountName string
	RuntimeClassName   string
}

type MessageTransferConfig struct {
	Name          string
	WorkerID      string
	DryRun        bool
	StorageDriver string
	DataSource    string
	Tracing       observability.TracingConfig
	Consumer      TransferConsumerConfig
	Dispatcher    TransferDispatcherConfig
	Worker        TransferWorkerConfig
	Observability ObservabilityHTTPConfig
	Kafka         TransferKafkaConfig
}

// TransferKafkaConfig enables the Kafka write pipeline (03-message-pipeline §9 B1):
// consume msg.toTransfer.v1, allocate seq in Redis, persist to PG via
// msg.toPostgres.v1 and fan out via msg.toPush.v1. Coexists with the legacy
// outbox consumer until 03 §9 B3 retires it.
type TransferKafkaConfig struct {
	Enabled bool
	// Brokers is a comma-separated bootstrap list, e.g. "redpanda.agents-im.svc.cluster.local:9092".
	Brokers string
	Redis   RedisConfig
	// Workers bounds parallel per-conversation batch processing (default 8).
	Workers int
	// TypedAccountIDs turns on the D16 toPush filter: receivers whose account id
	// carries the agent type bits are dropped from push fanout (D15 migration
	// step ②). MUST stay false until the D16 account-id switch + data reset
	// (step ①) has happened — legacy snowflake ids decode the type bits as noise
	// and would randomly lose pushes. Env override: MESSAGE_TRANSFER_TYPED_ACCOUNT_IDS.
	TypedAccountIDs bool
}

type TransferConsumerConfig struct {
	Driver string
	Topic  string
	Group  string
}

type TransferDispatcherConfig struct {
	Driver          string
	GatewayEndpoint string
}

type TransferWorkerConfig struct {
	PollIntervalMillis int64
	RetryBackoffMillis int64
	MaxAttempts        int
}

type ObservabilityHTTPConfig struct {
	Enabled bool
	Host    string
	Port    int
}

const (
	StorageDriverMemory             = "memory"
	StorageDriverPostgres           = "postgres"
	ObjectStorageDriverMemory       = "memory"
	ObjectStorageDriverMinIO        = "minio"
	PresenceDriverMemory            = "memory"
	PresenceDriverRedis             = "redis"
	TransferConsumerMemory          = "memory"
	TransferConsumerOutbox          = "outbox"
	TransferDispatcherNoop          = "noop"
	TransferDispatcherGateway       = "gateway"
	LLMObservabilityBackendNoop     = "noop"
	LLMObservabilityBackendMemory   = "memory"
	LLMObservabilityBackendTest     = "test"
	LLMObservabilityBackendLangfuse = "langfuse"
	PythonExecutorBackendDisabled   = "disabled"
	PythonExecutorBackendK8S        = "k8s"
)

const (
	defaultRedisAddr                   = "localhost:6379"
	defaultPresenceHeartbeatTTLSeconds = 60
	defaultPresenceRedisKeyPrefix      = "agents_im:presence"
	defaultGatewayWSPingSeconds        = 30
	defaultGatewayWSHeartbeatSeconds   = 75
	defaultGatewayWSCommandRate        = 20
	defaultGatewayWSCommandBurst       = 40
	defaultObjectStorageBucket         = "agents-im-media"
	defaultObjectStorageRegion         = "us-east-1"
	defaultTransferTopic               = "message.events.v1"
	defaultTransferGroup               = "msgtransfer-worker"
	defaultTransferPollIntervalMillis  = 100
	defaultTransferRetryBackoffMillis  = 1000
	defaultTransferMaxAttempts         = 5
	defaultTransferObservabilityHost   = "0.0.0.0"
	defaultTransferObservabilityPort   = 8085
	defaultLLMObservabilityMaxOutput   = 2048
	defaultPythonExecutorTimeout       = 10
	defaultPythonExecutorMaxTimeout    = 30
	defaultPythonExecutorMemoryMiB     = 256
	defaultPythonExecutorMaxOutput     = 64 * 1024
	DefaultDeepSeekBaseURL             = "https://api.deepseek.com"
	DefaultDeepSeekModel               = "deepseek-v4-pro"
	DefaultLangfuseHost                = "https://langfuse.agenticim.xyz"
)

var ErrDeepSeekAPIKeyMissing = errors.New("deepseek API key is required: set DEEPSEEK_API_KEY")
var ErrDeepSeekAPIKeyPlaceholder = errors.New("deepseek API key is a placeholder: set a real DEEPSEEK_API_KEY")
var ErrObjectStorageExternalEndpointLoopback = errors.New("object storage external endpoint cannot be loopback in production")

func DefaultAPIConfig() APIConfig {
	return APIConfig{
		Name:             "user-api",
		Host:             "0.0.0.0",
		Port:             8080,
		Auth:             DefaultJWTAuthConfig(),
		AdminBootstrap:   DefaultAdminBootstrapConfig(),
		StorageDriver:    StorageDriverMemory,
		Redis:            DefaultRedisConfig(),
		Presence:         DefaultPresenceConfig(),
		Tracing:          observability.TracingConfig{},
		DeepSeek:         DefaultDeepSeekConfig(),
		LLMObservability: DefaultLLMObservabilityConfig(),
		GatewayWS:        DefaultGatewayWSConfig(),
		ObjectStorage:    DefaultObjectStorageConfig(),
		PythonExecutor:   DefaultPythonExecutorConfig(),
	}
}

func DefaultRPCConfig() RPCConfig {
	return RPCConfig{
		Name:          "user-rpc",
		ListenOn:      "0.0.0.0:9090",
		Auth:          DefaultJWTAuthConfig(),
		StorageDriver: StorageDriverMemory,
		Redis:         DefaultRedisConfig(),
		Presence:      DefaultPresenceConfig(),
		Tracing:       observability.TracingConfig{},
	}
}

func DefaultJWTAuthConfig() JWTAuthConfig {
	return JWTAuthConfig{
		AccessSecret: "dev-jwt-secret-change-me",
		AccessExpire: 86400,
	}
}

func DefaultAdminBootstrapConfig() AdminBootstrapConfig {
	return AdminBootstrapConfig{
		Identifier:  "amin",
		DisplayName: "管理后台管理员",
	}
}

func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		Addr: defaultRedisAddr,
		DB:   0,
	}
}

func DefaultPresenceConfig() PresenceConfig {
	return PresenceConfig{
		Driver:              PresenceDriverMemory,
		HeartbeatTTLSeconds: defaultPresenceHeartbeatTTLSeconds,
		KeyPrefix:           defaultPresenceRedisKeyPrefix,
	}
}

func DefaultGatewayWSConfig() GatewayWSConfig {
	return GatewayWSConfig{
		AllowedOrigins:            nil,
		AllowQueryToken:           false,
		PingIntervalSeconds:       defaultGatewayWSPingSeconds,
		HeartbeatTimeoutSeconds:   defaultGatewayWSHeartbeatSeconds,
		CommandRateLimitPerSecond: defaultGatewayWSCommandRate,
		CommandRateLimitBurst:     defaultGatewayWSCommandBurst,
	}
}

func DefaultDeepSeekConfig() DeepSeekConfig {
	return DeepSeekConfig{
		BaseURL: DefaultDeepSeekBaseURL,
		Model:   DefaultDeepSeekModel,
	}
}

func DefaultLLMObservabilityConfig() LLMObservabilityConfig {
	return LLMObservabilityConfig{
		Enabled:        false,
		Backend:        LLMObservabilityBackendNoop,
		MaxOutputBytes: defaultLLMObservabilityMaxOutput,
		Langfuse: LangfuseObservabilityConfig{
			Host: DefaultLangfuseHost,
		},
	}
}

func DefaultObjectStorageConfig() ObjectStorageConfig {
	return ObjectStorageConfig{
		Driver: ObjectStorageDriverMemory,
		Bucket: defaultObjectStorageBucket,
		Region: defaultObjectStorageRegion,
	}
}

func DefaultPythonExecutorConfig() PythonExecutorConfig {
	return PythonExecutorConfig{
		Backend:               PythonExecutorBackendDisabled,
		DefaultTimeoutSeconds: defaultPythonExecutorTimeout,
		MaxTimeoutSeconds:     defaultPythonExecutorMaxTimeout,
		DefaultMemoryMiB:      defaultPythonExecutorMemoryMiB,
		MaxMemoryMiB:          defaultPythonExecutorMemoryMiB,
		MaxOutputBytes:        defaultPythonExecutorMaxOutput,
	}
}

func DefaultMessageTransferConfig() MessageTransferConfig {
	return MessageTransferConfig{
		Name:          "msgtransfer",
		WorkerID:      defaultWorkerID(),
		DryRun:        true,
		StorageDriver: StorageDriverMemory,
		Tracing:       observability.TracingConfig{},
		Consumer: TransferConsumerConfig{
			Driver: TransferConsumerMemory,
			Topic:  defaultTransferTopic,
			Group:  defaultTransferGroup,
		},
		Dispatcher: TransferDispatcherConfig{
			Driver: TransferDispatcherNoop,
		},
		Worker: TransferWorkerConfig{
			PollIntervalMillis: defaultTransferPollIntervalMillis,
			RetryBackoffMillis: defaultTransferRetryBackoffMillis,
			MaxAttempts:        defaultTransferMaxAttempts,
		},
		Observability: ObservabilityHTTPConfig{
			Enabled: true,
			Host:    defaultTransferObservabilityHost,
			Port:    defaultTransferObservabilityPort,
		},
	}
}

func LoadAPIConfig(path string) (APIConfig, error) {
	cfg := DefaultAPIConfig()
	values, err := readFlatYAML(path)
	if err != nil {
		return cfg, err
	}

	if value := values["Name"]; value != "" {
		cfg.Name = value
	}
	if value := values["Host"]; value != "" {
		cfg.Host = value
	}
	if value := values["Port"]; value != "" {
		port, err := strconv.Atoi(value)
		if err != nil {
			return cfg, err
		}
		cfg.Port = port
	}
	if value := strings.TrimSpace(os.ExpandEnv(values["Auth.AccessSecret"])); value != "" {
		cfg.Auth.AccessSecret = value
	}
	if value := strings.TrimSpace(os.ExpandEnv(values["Auth.AccessExpire"])); value != "" {
		expire, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return cfg, err
		}
		cfg.Auth.AccessExpire = expire
	}
	cfg.AdminBootstrap = adminBootstrapConfigFromValues(values)
	if value := firstNonEmpty(values["StorageDriver"], values["Repository"]); value != "" {
		cfg.StorageDriver = ResolveStorageDriver(value)
	} else {
		cfg.StorageDriver = ResolveStorageDriver(cfg.StorageDriver)
	}
	cfg.DataSource = ResolveDataSource(values["DataSource"])
	cfg.Redis, err = redisConfigFromValues(values)
	if err != nil {
		return cfg, err
	}
	cfg.Presence, err = presenceConfigFromValues(values)
	if err != nil {
		return cfg, err
	}
	cfg.GatewayWS, err = gatewayWSConfigFromValues(values)
	if err != nil {
		return cfg, err
	}
	cfg.Tracing, err = tracingConfigFromValues(values, cfg.Tracing, cfg.Name)
	if err != nil {
		return cfg, err
	}
	cfg.DeepSeek = deepSeekConfigFromValues(values)
	cfg.LLMObservability, err = llmObservabilityConfigFromValues(values)
	if err != nil {
		return cfg, err
	}
	cfg.ObjectStorage, err = objectStorageConfigFromValues(values, cfg.StorageDriver)
	if err != nil {
		return cfg, err
	}
	cfg.PythonExecutor, err = pythonExecutorConfigFromValues(values)
	if err != nil {
		return cfg, err
	}
	cfg.MailRPC, err = mailRPCConfigFromValues(values)
	if err != nil {
		return cfg, err
	}
	cfg.MsgRPC, err = msgRPCConfigFromValues(values)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

func LoadRPCConfig(path string) (RPCConfig, error) {
	cfg := DefaultRPCConfig()
	values, err := readFlatYAML(path)
	if err != nil {
		return cfg, err
	}

	if value := values["Name"]; value != "" {
		cfg.Name = value
	}
	if value := values["ListenOn"]; value != "" {
		cfg.ListenOn = value
	}
	if value := strings.TrimSpace(os.ExpandEnv(values["Auth.AccessSecret"])); value != "" {
		cfg.Auth.AccessSecret = value
	}
	if value := strings.TrimSpace(os.ExpandEnv(values["Auth.AccessExpire"])); value != "" {
		expire, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return cfg, err
		}
		cfg.Auth.AccessExpire = expire
	}
	if value := firstNonEmpty(values["StorageDriver"], values["Repository"]); value != "" {
		cfg.StorageDriver = ResolveStorageDriver(value)
	} else {
		cfg.StorageDriver = ResolveStorageDriver(cfg.StorageDriver)
	}
	cfg.DataSource = ResolveDataSource(values["DataSource"])
	cfg.Redis, err = redisConfigFromValues(values)
	if err != nil {
		return cfg, err
	}
	cfg.Presence, err = presenceConfigFromValues(values)
	if err != nil {
		return cfg, err
	}
	cfg.Tracing, err = tracingConfigFromValues(values, cfg.Tracing, cfg.Name)
	if err != nil {
		return cfg, err
	}
	cfg.MailRPC, err = mailRPCConfigFromValues(values)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

func LoadMessageTransferConfig(path string) (MessageTransferConfig, error) {
	cfg := DefaultMessageTransferConfig()
	values, err := readFlatYAML(path)
	if err != nil {
		return cfg, err
	}

	if value := values["Name"]; value != "" {
		cfg.Name = value
	}
	cfg.WorkerID = firstNonEmpty(values["Worker.ID"], values["WorkerID"], os.Getenv("MESSAGE_TRANSFER_WORKER_ID"), cfg.WorkerID)
	if value := firstNonEmpty(values["StorageDriver"], values["Repository"]); value != "" {
		cfg.StorageDriver = ResolveStorageDriver(value)
	} else {
		cfg.StorageDriver = ResolveStorageDriver(cfg.StorageDriver)
	}
	cfg.DataSource = ResolveDataSource(values["DataSource"])
	cfg.Tracing, err = tracingConfigFromValues(values, cfg.Tracing, cfg.Name)
	if err != nil {
		return cfg, err
	}
	if value := firstNonEmpty(values["Consumer.Driver"], values["ConsumerDriver"]); value != "" {
		cfg.Consumer.Driver = ResolveTransferConsumerDriver(value)
	} else {
		cfg.Consumer.Driver = ResolveTransferConsumerDriver(cfg.Consumer.Driver)
	}
	if value := firstNonEmpty(values["Consumer.Topic"], values["Topic"]); value != "" {
		cfg.Consumer.Topic = strings.TrimSpace(os.ExpandEnv(value))
	} else {
		cfg.Consumer.Topic = ""
	}
	if value := firstNonEmpty(values["Consumer.Group"], values["ConsumerGroup"]); value != "" {
		cfg.Consumer.Group = strings.TrimSpace(os.ExpandEnv(value))
	} else {
		cfg.Consumer.Group = ""
	}
	if value := firstNonEmpty(values["Dispatcher.Driver"], values["DispatcherDriver"]); value != "" {
		cfg.Dispatcher.Driver = ResolveTransferDispatcherDriver(value)
	} else {
		cfg.Dispatcher.Driver = ResolveTransferDispatcherDriver(cfg.Dispatcher.Driver)
	}
	cfg.Dispatcher.GatewayEndpoint = firstNonEmpty(
		strings.TrimSpace(os.ExpandEnv(values["Dispatcher.GatewayEndpoint"])),
		strings.TrimSpace(os.ExpandEnv(values["Dispatcher.Endpoint"])),
		os.Getenv("MESSAGE_TRANSFER_GATEWAY_ENDPOINT"),
		cfg.Dispatcher.GatewayEndpoint,
	)
	if value := firstNonEmpty(values["Worker.PollIntervalMillis"], values["PollIntervalMillis"]); value != "" {
		interval, err := strconv.ParseInt(strings.TrimSpace(os.ExpandEnv(value)), 10, 64)
		if err != nil {
			return cfg, err
		}
		cfg.Worker.PollIntervalMillis = interval
	}
	if value := firstNonEmpty(values["Worker.RetryBackoffMillis"], values["RetryBackoffMillis"]); value != "" {
		backoff, err := strconv.ParseInt(strings.TrimSpace(os.ExpandEnv(value)), 10, 64)
		if err != nil {
			return cfg, err
		}
		cfg.Worker.RetryBackoffMillis = backoff
	}
	if value := firstNonEmpty(values["Worker.MaxAttempts"], values["MaxAttempts"]); value != "" {
		maxAttempts, err := strconv.Atoi(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.Worker.MaxAttempts = maxAttempts
	}
	if value := values["DryRun"]; value != "" {
		dryRun, err := strconv.ParseBool(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.DryRun = dryRun
	}
	cfg.Observability, err = observabilityHTTPConfigFromValues(values, cfg.Observability)
	if err != nil {
		return cfg, err
	}
	// Kafka 写链路（03 §9 B1/B2）。注意本加载器是 flat-YAML 白名单制：新增配置段
	// 必须在这里显式读取，否则 yaml 配置会被静默丢弃（#480 教训）。
	if value := values["Kafka.Enabled"]; value != "" {
		enabled, err := strconv.ParseBool(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.Kafka.Enabled = enabled
	}
	cfg.Kafka.Brokers = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(values["Kafka.Brokers"])), cfg.Kafka.Brokers)
	if value := values["Kafka.Workers"]; value != "" {
		workers, err := strconv.Atoi(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.Kafka.Workers = workers
	}
	cfg.Kafka.Redis.Addr = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(values["Kafka.Redis.Addr"])), cfg.Kafka.Redis.Addr)
	cfg.Kafka.Redis.Password = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(values["Kafka.Redis.Password"])), cfg.Kafka.Redis.Password)
	if value := values["Kafka.Redis.DB"]; value != "" {
		db, err := strconv.Atoi(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.Kafka.Redis.DB = db
	}
	if value := values["Kafka.TypedAccountIds"]; value != "" {
		typed, err := strconv.ParseBool(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.Kafka.TypedAccountIDs = typed
	}
	cfg = ResolveMessageTransferConfig(cfg)
	return cfg, nil
}

func ToRestConf(cfg APIConfig) rest.RestConf {
	var restConf rest.RestConf
	restConf.Name = cfg.Name
	restConf.Host = cfg.Host
	restConf.Port = cfg.Port
	restConf.Telemetry = GoZeroTelemetryConfig(cfg.Tracing, cfg.Name)
	return restConf
}

func GoZeroTelemetryConfig(cfg observability.TracingConfig, serviceName string) gozerotrace.Config {
	resolved, err := observability.ResolveTracingConfig(cfg, serviceName)
	if err != nil {
		return gozerotrace.Config{Name: strings.TrimSpace(serviceName), Disabled: true}
	}
	telemetry := gozerotrace.Config{
		Name:     resolved.ServiceName,
		Endpoint: resolved.OTLPEndpoint,
		Sampler:  resolved.SamplerRatio,
		Disabled: !resolved.Enabled,
	}
	switch resolved.Protocol {
	case observability.TracingProtocolHTTP:
		telemetry.Batcher = "otlphttp"
		telemetry.OtlpHttpSecure = !resolved.Insecure
	default:
		telemetry.Batcher = "otlpgrpc"
	}
	return telemetry
}

func ResolveStorageDriver(value string) string {
	value = strings.ToLower(strings.TrimSpace(os.ExpandEnv(value)))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(os.Getenv("AGENTS_IM_STORAGE_DRIVER")))
	}
	switch value {
	case "", StorageDriverMemory:
		return StorageDriverMemory
	case StorageDriverPostgres:
		return StorageDriverPostgres
	default:
		return value
	}
}

func ResolvePresenceDriver(value string) string {
	value = strings.ToLower(strings.TrimSpace(os.ExpandEnv(value)))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(os.Getenv("PRESENCE_DRIVER")))
	}
	switch value {
	case PresenceDriverRedis:
		return PresenceDriverRedis
	default:
		return PresenceDriverMemory
	}
}

func ResolveTransferConsumerDriver(value string) string {
	value = strings.ToLower(strings.TrimSpace(os.ExpandEnv(value)))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(os.Getenv("MESSAGE_TRANSFER_CONSUMER_DRIVER")))
	}
	switch value {
	case TransferConsumerOutbox, "postgres_outbox", "postgres-outbox":
		return TransferConsumerOutbox
	default:
		return TransferConsumerMemory
	}
}

func ResolveTransferDispatcherDriver(value string) string {
	value = strings.ToLower(strings.TrimSpace(os.ExpandEnv(value)))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(os.Getenv("MESSAGE_TRANSFER_DISPATCHER_DRIVER")))
	}
	switch value {
	case TransferDispatcherGateway, "gateway-http", "http":
		return TransferDispatcherGateway
	default:
		return TransferDispatcherNoop
	}
}

func ResolveMessageTransferConfig(cfg MessageTransferConfig) MessageTransferConfig {
	cfg.Name = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Name)), "msgtransfer")
	cfg.WorkerID = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.WorkerID)), os.Getenv("MESSAGE_TRANSFER_WORKER_ID"), defaultWorkerID())
	cfg.StorageDriver = ResolveStorageDriver(cfg.StorageDriver)
	cfg.DataSource = ResolveDataSource(cfg.DataSource)
	if tracing, err := observability.ResolveTracingConfig(cfg.Tracing, cfg.Name); err == nil {
		cfg.Tracing = tracing
	}
	cfg.Consumer.Driver = ResolveTransferConsumerDriver(cfg.Consumer.Driver)
	cfg.Consumer.Topic = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Consumer.Topic)), os.Getenv("MESSAGE_TRANSFER_TOPIC"), defaultTransferTopic)
	cfg.Consumer.Group = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Consumer.Group)), os.Getenv("MESSAGE_TRANSFER_CONSUMER_GROUP"), defaultTransferGroup)
	cfg.Dispatcher.Driver = ResolveTransferDispatcherDriver(cfg.Dispatcher.Driver)
	cfg.Dispatcher.GatewayEndpoint = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Dispatcher.GatewayEndpoint)), os.Getenv("MESSAGE_TRANSFER_GATEWAY_ENDPOINT"))
	if cfg.Worker.PollIntervalMillis <= 0 {
		cfg.Worker.PollIntervalMillis = defaultTransferPollIntervalMillis
	}
	if cfg.Worker.RetryBackoffMillis <= 0 {
		cfg.Worker.RetryBackoffMillis = defaultTransferRetryBackoffMillis
	}
	if cfg.Worker.MaxAttempts <= 0 {
		cfg.Worker.MaxAttempts = defaultTransferMaxAttempts
	}
	resolvedObservability, err := ResolveObservabilityHTTPConfig(cfg.Observability)
	if err == nil {
		cfg.Observability = resolvedObservability
	}
	cfg.Kafka = resolveTransferKafkaConfig(cfg.Kafka)
	return cfg
}

func resolveTransferKafkaConfig(cfg TransferKafkaConfig) TransferKafkaConfig {
	if value := strings.TrimSpace(os.Getenv("MESSAGE_TRANSFER_KAFKA_ENABLED")); value != "" {
		if enabled, err := strconv.ParseBool(value); err == nil {
			cfg.Enabled = enabled
		}
	}
	cfg.Brokers = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Brokers)), os.Getenv("KAFKA_BROKERS"))
	if value := strings.TrimSpace(os.Getenv("MESSAGE_TRANSFER_TYPED_ACCOUNT_IDS")); value != "" {
		if typed, err := strconv.ParseBool(value); err == nil {
			cfg.TypedAccountIDs = typed
		}
	}
	if resolved, err := ResolveRedisConfig(cfg.Redis); err == nil {
		cfg.Redis = resolved
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 8
	}
	return cfg
}

// KafkaBrokerList splits the comma-separated broker string.
func KafkaBrokerList(brokers string) []string {
	parts := strings.Split(brokers, ",")
	list := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			list = append(list, trimmed)
		}
	}
	return list
}

func ResolveObservabilityHTTPConfig(cfg ObservabilityHTTPConfig) (ObservabilityHTTPConfig, error) {
	if value := firstNonEmpty(os.Getenv("MESSAGE_TRANSFER_OBSERVABILITY_ENABLED"), os.Getenv("AGENTS_IM_OBSERVABILITY_ENABLED")); value != "" {
		enabled, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return cfg, err
		}
		cfg.Enabled = enabled
	}
	cfg.Host = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Host)), os.Getenv("MESSAGE_TRANSFER_OBSERVABILITY_HOST"), defaultTransferObservabilityHost)
	port, err := resolveInt(cfg.Port, os.Getenv("MESSAGE_TRANSFER_OBSERVABILITY_PORT"))
	if err != nil {
		return cfg, err
	}
	if port <= 0 {
		port = defaultTransferObservabilityPort
	}
	cfg.Port = port
	return cfg, nil
}

func ResolveDataSource(value string) string {
	value = strings.TrimSpace(os.ExpandEnv(value))
	if value != "" {
		return value
	}
	for _, key := range []string{"DATABASE_URL", "AGENTS_IM_POSTGRES_DSN", "POSTGRES_DSN"} {
		if envValue := strings.TrimSpace(os.Getenv(key)); envValue != "" {
			return envValue
		}
	}
	return ""
}

func ResolveRedisConfig(cfg RedisConfig) (RedisConfig, error) {
	cfg.Addr = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Addr)), os.Getenv("REDIS_ADDR"), os.Getenv("AGENTS_IM_REDIS_ADDR"), defaultRedisAddr)
	cfg.Password = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Password)), os.Getenv("REDIS_PASSWORD"), os.Getenv("AGENTS_IM_REDIS_PASSWORD"))

	db, err := resolveInt(cfg.DB, os.Getenv("REDIS_DB"), os.Getenv("AGENTS_IM_REDIS_DB"))
	if err != nil {
		return cfg, err
	}
	cfg.DB = db
	return cfg, nil
}

func ResolvePresenceConfig(cfg PresenceConfig) (PresenceConfig, error) {
	cfg.Driver = ResolvePresenceDriver(cfg.Driver)

	ttl, err := resolveInt64(cfg.HeartbeatTTLSeconds, os.Getenv("PRESENCE_TTL_SECONDS"), os.Getenv("AGENTS_IM_PRESENCE_TTL_SECONDS"))
	if err != nil {
		return cfg, err
	}
	if ttl <= 0 {
		ttl = defaultPresenceHeartbeatTTLSeconds
	}
	cfg.HeartbeatTTLSeconds = ttl
	cfg.KeyPrefix = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.KeyPrefix)), os.Getenv("PRESENCE_KEY_PREFIX"), os.Getenv("AGENTS_IM_PRESENCE_KEY_PREFIX"), defaultPresenceRedisKeyPrefix)
	return cfg, nil
}

func ResolveGatewayWSConfig(cfg GatewayWSConfig) (GatewayWSConfig, error) {
	origins := cfg.AllowedOrigins
	if len(origins) == 0 {
		origins = originListFromValue(firstNonEmpty(os.Getenv("GATEWAY_WS_ALLOWED_ORIGINS"), os.Getenv("AGENTS_IM_GATEWAY_WS_ALLOWED_ORIGINS")))
	}
	normalizedOrigins := make([]string, 0, len(origins))
	seenOrigins := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		normalized, err := normalizeOrigin(origin)
		if err != nil {
			return cfg, err
		}
		if normalized == "" {
			continue
		}
		if _, ok := seenOrigins[normalized]; ok {
			continue
		}
		seenOrigins[normalized] = struct{}{}
		normalizedOrigins = append(normalizedOrigins, normalized)
	}
	cfg.AllowedOrigins = normalizedOrigins

	allowQueryToken, err := resolveBool(cfg.AllowQueryToken, os.Getenv("GATEWAY_WS_ALLOW_QUERY_TOKEN"), os.Getenv("AGENTS_IM_GATEWAY_WS_ALLOW_QUERY_TOKEN"))
	if err != nil {
		return cfg, err
	}
	cfg.AllowQueryToken = allowQueryToken

	pingInterval, err := resolveInt64(cfg.PingIntervalSeconds, os.Getenv("GATEWAY_WS_PING_INTERVAL_SECONDS"), os.Getenv("AGENTS_IM_GATEWAY_WS_PING_INTERVAL_SECONDS"))
	if err != nil {
		return cfg, err
	}
	if pingInterval <= 0 {
		pingInterval = defaultGatewayWSPingSeconds
	}
	cfg.PingIntervalSeconds = pingInterval

	heartbeatTimeout, err := resolveInt64(cfg.HeartbeatTimeoutSeconds, os.Getenv("GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS"), os.Getenv("AGENTS_IM_GATEWAY_WS_HEARTBEAT_TIMEOUT_SECONDS"))
	if err != nil {
		return cfg, err
	}
	if heartbeatTimeout <= 0 {
		heartbeatTimeout = defaultGatewayWSHeartbeatSeconds
	}
	cfg.HeartbeatTimeoutSeconds = heartbeatTimeout
	if cfg.PingIntervalSeconds >= cfg.HeartbeatTimeoutSeconds {
		cfg.PingIntervalSeconds = maxInt64(1, cfg.HeartbeatTimeoutSeconds/2)
	}

	commandRate, err := resolveInt(cfg.CommandRateLimitPerSecond, os.Getenv("GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND"), os.Getenv("AGENTS_IM_GATEWAY_WS_COMMAND_RATE_LIMIT_PER_SECOND"))
	if err != nil {
		return cfg, err
	}
	if commandRate <= 0 {
		commandRate = defaultGatewayWSCommandRate
	}
	cfg.CommandRateLimitPerSecond = commandRate

	commandBurst, err := resolveInt(cfg.CommandRateLimitBurst, os.Getenv("GATEWAY_WS_COMMAND_RATE_LIMIT_BURST"), os.Getenv("AGENTS_IM_GATEWAY_WS_COMMAND_RATE_LIMIT_BURST"))
	if err != nil {
		return cfg, err
	}
	if commandBurst <= 0 {
		commandBurst = maxInt(defaultGatewayWSCommandBurst, commandRate)
	}
	cfg.CommandRateLimitBurst = commandBurst
	return cfg, nil
}

func ResolveDeepSeekConfig(cfg DeepSeekConfig) DeepSeekConfig {
	cfg.APIKey = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.APIKey)), os.Getenv("DEEPSEEK_API_KEY"))
	cfg.BaseURL = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.BaseURL)), os.Getenv("DEEPSEEK_BASE_URL"), DefaultDeepSeekBaseURL)
	cfg.Model = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Model)), os.Getenv("DEEPSEEK_MODEL"), DefaultDeepSeekModel)
	return cfg
}

func ResolveLLMObservabilityConfig(cfg LLMObservabilityConfig) (LLMObservabilityConfig, error) {
	enabled, err := resolveBool(cfg.Enabled, os.Getenv("LLM_OBSERVABILITY_ENABLED"), os.Getenv("LLM_OBS_ENABLED"), os.Getenv("AGENTS_IM_LLM_OBSERVABILITY_ENABLED"))
	if err != nil {
		return cfg, err
	}
	cfg.Enabled = enabled
	cfg.Backend, err = resolveLLMObservabilityBackend(cfg.Backend)
	if err != nil {
		return cfg, err
	}
	cfg.CaptureOutput, err = resolveBool(cfg.CaptureOutput, os.Getenv("LLM_OBSERVABILITY_CAPTURE_OUTPUT"), os.Getenv("LLM_OBS_CAPTURE_OUTPUT"))
	if err != nil {
		return cfg, err
	}
	maxOutputBytes, err := resolveLLMObservabilityMaxOutputBytes(cfg.MaxOutputBytes)
	if err != nil {
		return cfg, err
	}
	if maxOutputBytes < 0 {
		maxOutputBytes = 0
	}
	if maxOutputBytes == 0 {
		maxOutputBytes = DefaultLLMObservabilityConfig().MaxOutputBytes
	}
	cfg.MaxOutputBytes = maxOutputBytes
	langfuseHost := strings.TrimSpace(os.ExpandEnv(cfg.Langfuse.Host))
	langfuseHostEnv := firstNonEmpty(os.Getenv("LANGFUSE_HOST"), os.Getenv("LANGFUSE_BASE_URL"))
	if langfuseHost == "" || langfuseHost == DefaultLangfuseHost {
		cfg.Langfuse.Host = firstNonEmpty(langfuseHostEnv, langfuseHost, DefaultLangfuseHost)
	} else {
		cfg.Langfuse.Host = langfuseHost
	}
	cfg.Langfuse.PublicKey = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Langfuse.PublicKey)), os.Getenv("LANGFUSE_PUBLIC_KEY"))
	cfg.Langfuse.SecretKey = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Langfuse.SecretKey)), os.Getenv("LANGFUSE_SECRET_KEY"))
	return cfg, nil
}

func ResolvePythonExecutorConfig(cfg PythonExecutorConfig) (PythonExecutorConfig, error) {
	defaults := DefaultPythonExecutorConfig()
	backend := strings.ToLower(strings.TrimSpace(os.ExpandEnv(cfg.Backend)))
	if backend == "" {
		backend = strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("PYTHON_EXECUTOR_BACKEND"), os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_BACKEND"))))
	}
	if backend == "" {
		backend = PythonExecutorBackendDisabled
	}
	switch backend {
	case PythonExecutorBackendDisabled, PythonExecutorBackendK8S:
		cfg.Backend = backend
	default:
		return cfg, fmt.Errorf("unsupported python executor backend %q; use %q or %q", backend, PythonExecutorBackendDisabled, PythonExecutorBackendK8S)
	}

	cfg.K8S.Namespace = firstNonEmpty(
		strings.TrimSpace(os.ExpandEnv(cfg.K8S.Namespace)),
		os.Getenv("PYTHON_EXECUTOR_K8S_NAMESPACE"),
		os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_K8S_NAMESPACE"),
	)
	cfg.K8S.Image = firstNonEmpty(
		strings.TrimSpace(os.ExpandEnv(cfg.K8S.Image)),
		os.Getenv("PYTHON_EXECUTOR_K8S_IMAGE"),
		os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_K8S_IMAGE"),
	)
	cfg.K8S.ServiceAccountName = firstNonEmpty(
		strings.TrimSpace(os.ExpandEnv(cfg.K8S.ServiceAccountName)),
		os.Getenv("PYTHON_EXECUTOR_K8S_SERVICE_ACCOUNT_NAME"),
		os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_K8S_SERVICE_ACCOUNT_NAME"),
	)
	cfg.K8S.RuntimeClassName = firstNonEmpty(
		strings.TrimSpace(os.ExpandEnv(cfg.K8S.RuntimeClassName)),
		os.Getenv("PYTHON_EXECUTOR_K8S_RUNTIME_CLASS_NAME"),
		os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_K8S_RUNTIME_CLASS_NAME"),
	)

	defaultTimeout, err := resolveInt(cfg.DefaultTimeoutSeconds, os.Getenv("PYTHON_EXECUTOR_DEFAULT_TIMEOUT_SECONDS"), os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_DEFAULT_TIMEOUT_SECONDS"))
	if err != nil {
		return cfg, err
	}
	if defaultTimeout <= 0 {
		defaultTimeout = defaults.DefaultTimeoutSeconds
	}
	cfg.DefaultTimeoutSeconds = defaultTimeout

	maxTimeout, err := resolveInt(cfg.MaxTimeoutSeconds, os.Getenv("PYTHON_EXECUTOR_MAX_TIMEOUT_SECONDS"), os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_MAX_TIMEOUT_SECONDS"))
	if err != nil {
		return cfg, err
	}
	if maxTimeout <= 0 {
		maxTimeout = defaults.MaxTimeoutSeconds
	}
	if maxTimeout < cfg.DefaultTimeoutSeconds {
		return cfg, fmt.Errorf("python executor max timeout must be greater than or equal to default timeout")
	}
	cfg.MaxTimeoutSeconds = maxTimeout

	defaultMemoryMiB, err := resolveInt(cfg.DefaultMemoryMiB, os.Getenv("PYTHON_EXECUTOR_DEFAULT_MEMORY_MIB"), os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_DEFAULT_MEMORY_MIB"))
	if err != nil {
		return cfg, err
	}
	if defaultMemoryMiB <= 0 {
		defaultMemoryMiB = defaults.DefaultMemoryMiB
	}
	cfg.DefaultMemoryMiB = defaultMemoryMiB

	maxMemoryMiB, err := resolveInt(cfg.MaxMemoryMiB, os.Getenv("PYTHON_EXECUTOR_MAX_MEMORY_MIB"), os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_MAX_MEMORY_MIB"))
	if err != nil {
		return cfg, err
	}
	if maxMemoryMiB <= 0 {
		maxMemoryMiB = cfg.DefaultMemoryMiB
	}
	if maxMemoryMiB < cfg.DefaultMemoryMiB {
		return cfg, fmt.Errorf("python executor max memory must be greater than or equal to default memory")
	}
	cfg.MaxMemoryMiB = maxMemoryMiB

	maxOutputBytes, err := resolveInt64(cfg.MaxOutputBytes, os.Getenv("PYTHON_EXECUTOR_MAX_OUTPUT_BYTES"), os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_MAX_OUTPUT_BYTES"))
	if err != nil {
		return cfg, err
	}
	if maxOutputBytes <= 0 {
		maxOutputBytes = defaults.MaxOutputBytes
	}
	cfg.MaxOutputBytes = maxOutputBytes

	if cfg.Backend == PythonExecutorBackendK8S {
		missing := make([]string, 0, 2)
		if strings.TrimSpace(cfg.K8S.Namespace) == "" {
			missing = append(missing, "namespace")
		}
		if strings.TrimSpace(cfg.K8S.Image) == "" {
			missing = append(missing, "image")
		}
		if len(missing) > 0 {
			return cfg, fmt.Errorf("python executor k8s backend requires %s", strings.Join(missing, " and "))
		}
	}
	return cfg, nil
}

func resolveLLMObservabilityBackend(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(os.ExpandEnv(value)))
	envValue := strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("LLM_OBSERVABILITY_BACKEND"), os.Getenv("LLM_OBS_BACKEND"), os.Getenv("AGENTS_IM_LLM_OBSERVABILITY_BACKEND"))))
	if value == "" || (value == LLMObservabilityBackendNoop && envValue != "") {
		value = envValue
	}
	if value == "" {
		return LLMObservabilityBackendNoop, nil
	}
	switch value {
	case LLMObservabilityBackendNoop:
		return LLMObservabilityBackendNoop, nil
	case LLMObservabilityBackendLangfuse:
		return LLMObservabilityBackendLangfuse, nil
	case LLMObservabilityBackendMemory:
		return LLMObservabilityBackendMemory, nil
	case LLMObservabilityBackendTest:
		return LLMObservabilityBackendTest, nil
	default:
		return "", fmt.Errorf("unsupported llm observability backend %q", value)
	}
}

func resolveLLMObservabilityMaxOutputBytes(current int) (int, error) {
	envValue := firstNonEmpty(os.Getenv("LLM_OBSERVABILITY_MAX_OUTPUT_BYTES"), os.Getenv("LLM_OBS_MAX_OUTPUT_BYTES"))
	if envValue != "" && (current == 0 || current == defaultLLMObservabilityMaxOutput) {
		return strconv.Atoi(strings.TrimSpace(envValue))
	}
	return resolveInt(current, envValue)
}

func ResolveObjectStorageConfig(cfg ObjectStorageConfig, storageDriver string) (ObjectStorageConfig, error) {
	driver := strings.ToLower(strings.TrimSpace(os.ExpandEnv(cfg.Driver)))
	if driver == "" {
		driver = strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("OBJECT_STORAGE_DRIVER"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_DRIVER"))))
	}
	if driver == "" {
		if ResolveStorageDriver(storageDriver) == StorageDriverMemory {
			driver = ObjectStorageDriverMemory
		} else {
			driver = ObjectStorageDriverMinIO
		}
	}
	switch driver {
	case ObjectStorageDriverMemory, ObjectStorageDriverMinIO:
		cfg.Driver = driver
	default:
		return cfg, fmt.Errorf("unsupported object storage driver %q; use %q for explicit dev/test memory mode or %q for MinIO/S3-compatible storage", driver, ObjectStorageDriverMemory, ObjectStorageDriverMinIO)
	}

	cfg.Endpoint = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Endpoint)), os.Getenv("OBJECT_STORAGE_ENDPOINT"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_ENDPOINT"))
	cfg.ExternalEndpoint = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.ExternalEndpoint)), os.Getenv("OBJECT_STORAGE_EXTERNAL_ENDPOINT"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_EXTERNAL_ENDPOINT"))
	cfg.Bucket = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Bucket)), os.Getenv("OBJECT_STORAGE_BUCKET"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_BUCKET"), defaultObjectStorageBucket)
	cfg.Region = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Region)), os.Getenv("OBJECT_STORAGE_REGION"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_REGION"), defaultObjectStorageRegion)
	cfg.AccessKeyID = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.AccessKeyID)), os.Getenv("OBJECT_STORAGE_ACCESS_KEY_ID"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_ACCESS_KEY_ID"), os.Getenv("MINIO_ROOT_USER"))
	cfg.SecretAccessKey = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.SecretAccessKey)), os.Getenv("OBJECT_STORAGE_SECRET_ACCESS_KEY"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_SECRET_ACCESS_KEY"), os.Getenv("MINIO_ROOT_PASSWORD"))

	useSSL, err := resolveBool(cfg.UseSSL, os.Getenv("OBJECT_STORAGE_USE_SSL"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_USE_SSL"))
	if err != nil {
		return cfg, err
	}
	cfg.UseSSL = useSSL

	externalUseSSLFallback := cfg.UseSSL
	if cfg.ExternalEndpoint != "" && cfg.ExternalEndpoint != cfg.Endpoint {
		externalUseSSLFallback = true
	}
	externalUseSSL, err := resolveOptionalBool(cfg.ExternalUseSSL, externalUseSSLFallback, os.Getenv("OBJECT_STORAGE_EXTERNAL_USE_SSL"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_EXTERNAL_USE_SSL"))
	if err != nil {
		return cfg, err
	}
	cfg.ExternalUseSSL = externalUseSSL
	if err := ValidateObjectStorageConfig(cfg, storageDriver); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func ValidateObjectStorageConfig(cfg ObjectStorageConfig, storageDriver string) error {
	if !IsProductionEnvironment() {
		return nil
	}
	if ResolveStorageDriver(storageDriver) != StorageDriverPostgres || cfg.Driver != ObjectStorageDriverMinIO {
		return nil
	}
	presignEndpoint := firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.ExternalEndpoint)), strings.TrimSpace(os.ExpandEnv(cfg.Endpoint)))
	if isBrowserLocalEndpoint(presignEndpoint) {
		return ErrObjectStorageExternalEndpointLoopback
	}
	return nil
}

func IsProductionEnvironment() bool {
	for _, key := range []string{"AGENTS_IM_ENV", "APP_ENV", "GO_ENV", "ENVIRONMENT"} {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "prod", "production":
			return true
		}
	}
	return false
}

func ValidateDeepSeekConfig(cfg DeepSeekConfig) error {
	cfg = ResolveDeepSeekConfig(cfg)
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return ErrDeepSeekAPIKeyMissing
	}
	if isPlaceholderDeepSeekAPIKey(apiKey) {
		return ErrDeepSeekAPIKeyPlaceholder
	}
	return nil
}

func isPlaceholderDeepSeekAPIKey(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "replace-with-local-deepseek-api-key",
		"replace-with-your-deepseek-api-key",
		"your-deepseek-api-key",
		"your_deepseek_api_key",
		"deepseek-api-key",
		"test-deepseek-api-key":
		return true
	default:
		return strings.Contains(normalized, "placeholder") || strings.HasPrefix(normalized, "replace-with-")
	}
}

func adminBootstrapConfigFromValues(values map[string]string) AdminBootstrapConfig {
	cfg := DefaultAdminBootstrapConfig()
	if value := firstNonEmpty(values["AdminBootstrap.Identifier"], os.Getenv("ADMIN_BOOTSTRAP_IDENTIFIER")); value != "" {
		cfg.Identifier = strings.TrimSpace(os.ExpandEnv(value))
	}
	if value := firstNonEmpty(values["AdminBootstrap.Password"], os.Getenv("ADMIN_BOOTSTRAP_PASSWORD")); value != "" {
		cfg.Password = os.ExpandEnv(value)
	}
	if value := firstNonEmpty(values["AdminBootstrap.DisplayName"], os.Getenv("ADMIN_BOOTSTRAP_DISPLAY_NAME")); value != "" {
		cfg.DisplayName = strings.TrimSpace(os.ExpandEnv(value))
	}
	return cfg
}

func redisConfigFromValues(values map[string]string) (RedisConfig, error) {
	cfg := RedisConfig{
		Addr:     firstNonEmpty(values["Redis.Addr"], values["RedisAddr"]),
		Password: firstNonEmpty(values["Redis.Password"], values["RedisPassword"]),
		DB:       0,
	}
	if value := firstNonEmpty(values["Redis.DB"], values["RedisDB"]); strings.TrimSpace(value) != "" {
		db, err := strconv.Atoi(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.DB = db
	}
	return ResolveRedisConfig(cfg)
}

func presenceConfigFromValues(values map[string]string) (PresenceConfig, error) {
	cfg := PresenceConfig{
		Driver:              firstNonEmpty(values["Presence.Driver"], values["PresenceDriver"]),
		HeartbeatTTLSeconds: 0,
		KeyPrefix:           firstNonEmpty(values["Presence.KeyPrefix"], values["PresenceKeyPrefix"]),
	}
	if value := firstNonEmpty(values["Presence.HeartbeatTTLSeconds"], values["PresenceTTLSeconds"]); strings.TrimSpace(value) != "" {
		ttl, err := strconv.ParseInt(strings.TrimSpace(os.ExpandEnv(value)), 10, 64)
		if err != nil {
			return cfg, err
		}
		cfg.HeartbeatTTLSeconds = ttl
	}
	return ResolvePresenceConfig(cfg)
}

func gatewayWSConfigFromValues(values map[string]string) (GatewayWSConfig, error) {
	cfg := GatewayWSConfig{
		AllowedOrigins: originListFromValue(firstNonEmpty(values["GatewayWS.AllowedOrigins"], values["GatewayWSAllowedOrigins"])),
	}
	if value := firstNonEmpty(values["GatewayWS.AllowQueryToken"], values["GatewayWSAllowQueryToken"]); strings.TrimSpace(value) != "" {
		allowQueryToken, err := strconv.ParseBool(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.AllowQueryToken = allowQueryToken
	}
	if value := firstNonEmpty(values["GatewayWS.PingIntervalSeconds"], values["GatewayWSPingIntervalSeconds"]); strings.TrimSpace(value) != "" {
		seconds, err := strconv.ParseInt(strings.TrimSpace(os.ExpandEnv(value)), 10, 64)
		if err != nil {
			return cfg, err
		}
		cfg.PingIntervalSeconds = seconds
	}
	if value := firstNonEmpty(values["GatewayWS.HeartbeatTimeoutSeconds"], values["GatewayWSHeartbeatTimeoutSeconds"]); strings.TrimSpace(value) != "" {
		seconds, err := strconv.ParseInt(strings.TrimSpace(os.ExpandEnv(value)), 10, 64)
		if err != nil {
			return cfg, err
		}
		cfg.HeartbeatTimeoutSeconds = seconds
	}
	if value := firstNonEmpty(values["GatewayWS.CommandRateLimitPerSecond"], values["GatewayWSCommandRateLimitPerSecond"]); strings.TrimSpace(value) != "" {
		limit, err := strconv.Atoi(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.CommandRateLimitPerSecond = limit
	}
	if value := firstNonEmpty(values["GatewayWS.CommandRateLimitBurst"], values["GatewayWSCommandRateLimitBurst"]); strings.TrimSpace(value) != "" {
		burst, err := strconv.Atoi(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.CommandRateLimitBurst = burst
	}
	return ResolveGatewayWSConfig(cfg)
}

func tracingConfigFromValues(values map[string]string, current observability.TracingConfig, serviceName string) (observability.TracingConfig, error) {
	cfg := current
	if value := firstNonEmpty(values["Tracing.Enabled"], values["TracingEnabled"]); strings.TrimSpace(value) != "" {
		enabled, err := strconv.ParseBool(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.Enabled = enabled
	}
	cfg.ServiceName = firstNonEmpty(values["Tracing.ServiceName"], values["TracingServiceName"], cfg.ServiceName)
	cfg.Environment = firstNonEmpty(values["Tracing.Environment"], values["TracingEnv"], cfg.Environment)
	cfg.OTLPEndpoint = firstNonEmpty(values["Tracing.OTLPEndpoint"], values["Tracing.Endpoint"], values["OTLPEndpoint"], cfg.OTLPEndpoint)
	cfg.Protocol = firstNonEmpty(values["Tracing.Protocol"], values["OTLPProtocol"], cfg.Protocol)
	if value := firstNonEmpty(values["Tracing.SamplerRatio"], values["TracingSamplerRatio"]); strings.TrimSpace(value) != "" {
		ratio, err := strconv.ParseFloat(strings.TrimSpace(os.ExpandEnv(value)), 64)
		if err != nil {
			return cfg, err
		}
		cfg.SamplerRatio = ratio
	}
	cfg.TraceUIBaseURL = firstNonEmpty(values["Tracing.TraceUIBaseURL"], values["TraceUIBaseURL"], cfg.TraceUIBaseURL)
	if value := firstNonEmpty(values["Tracing.Insecure"], values["TracingInsecure"]); strings.TrimSpace(value) != "" {
		insecure, err := strconv.ParseBool(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.Insecure = insecure
	}
	return observability.ResolveTracingConfig(cfg, serviceName)
}

func mailRPCConfigFromValues(values map[string]string) (zrpc.RpcClientConf, error) {
	cfg := zrpc.RpcClientConf{
		Target:    firstNonEmpty(values["MailRPC.Target"], values["MailRPCTarget"]),
		Endpoints: brokerListFromValue(firstNonEmpty(values["MailRPC.Endpoints"], values["MailRPCEndpoints"])),
	}
	if value := firstNonEmpty(values["MailRPC.Timeout"], values["MailRPCTimeout"]); strings.TrimSpace(value) != "" {
		timeout, err := strconv.ParseInt(strings.TrimSpace(os.ExpandEnv(value)), 10, 64)
		if err != nil {
			return cfg, err
		}
		cfg.Timeout = timeout
	}
	return ResolveMailRPCConfig(cfg), nil
}

func msgRPCConfigFromValues(values map[string]string) (zrpc.RpcClientConf, error) {
	cfg := zrpc.RpcClientConf{
		Target:    firstNonEmpty(values["MsgRPC.Target"], values["MsgRPCTarget"]),
		Endpoints: brokerListFromValue(firstNonEmpty(values["MsgRPC.Endpoints"], values["MsgRPCEndpoints"])),
	}
	if value := firstNonEmpty(values["MsgRPC.Timeout"], values["MsgRPCTimeout"]); strings.TrimSpace(value) != "" {
		timeout, err := strconv.ParseInt(strings.TrimSpace(os.ExpandEnv(value)), 10, 64)
		if err != nil {
			return cfg, err
		}
		cfg.Timeout = timeout
	}
	return ResolveMsgRPCConfig(cfg), nil
}

// ResolveMsgRPCConfig 解析 msggateway → msg-rpc 客户端配置；env 兜底
// MSG_RPC_TARGET / AGENTS_IM_MSG_RPC_TARGET（同 MailRPC 模式）。
func ResolveMsgRPCConfig(cfg zrpc.RpcClientConf) zrpc.RpcClientConf {
	cfg.Target = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Target)), os.Getenv("AGENTS_IM_MSG_RPC_TARGET"), os.Getenv("MSG_RPC_TARGET"))
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = brokerListFromValue(firstNonEmpty(os.Getenv("AGENTS_IM_MSG_RPC_ENDPOINTS"), os.Getenv("MSG_RPC_ENDPOINTS")))
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5000
	}
	cfg.NonBlock = true
	return cfg
}

func ResolveMailRPCConfig(cfg zrpc.RpcClientConf) zrpc.RpcClientConf {
	cfg.Target = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Target)), os.Getenv("AUTH_MAIL_RPC_TARGET"), os.Getenv("AGENTS_IM_MAIL_RPC_TARGET"), os.Getenv("MAIL_RPC_TARGET"))
	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = brokerListFromValue(firstNonEmpty(os.Getenv("AUTH_MAIL_RPC_ENDPOINTS"), os.Getenv("AGENTS_IM_MAIL_RPC_ENDPOINTS"), os.Getenv("MAIL_RPC_ENDPOINTS")))
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2000
	}
	cfg.NonBlock = true
	return cfg
}

func deepSeekConfigFromValues(values map[string]string) DeepSeekConfig {
	cfg := DeepSeekConfig{
		APIKey:  firstNonEmpty(values["DeepSeek.APIKey"], values["DeepSeekAPIKey"]),
		BaseURL: firstNonEmpty(values["DeepSeek.BaseURL"], values["DeepSeekBaseURL"]),
		Model:   firstNonEmpty(values["DeepSeek.Model"], values["DeepSeekModel"]),
	}
	return ResolveDeepSeekConfig(cfg)
}

func llmObservabilityConfigFromValues(values map[string]string) (LLMObservabilityConfig, error) {
	cfg := DefaultLLMObservabilityConfig()
	if value := firstNonEmpty(values["LLMObservability.Enabled"], values["LLMObs.Enabled"]); strings.TrimSpace(value) != "" {
		enabled, err := strconv.ParseBool(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.Enabled = enabled
	}
	cfg.Backend = firstNonEmpty(values["LLMObservability.Backend"], values["LLMObs.Backend"], cfg.Backend)
	if value := firstNonEmpty(values["LLMObservability.CaptureOutput"], values["LLMObs.CaptureOutput"]); strings.TrimSpace(value) != "" {
		captureOutput, err := strconv.ParseBool(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.CaptureOutput = captureOutput
	}
	if value := firstNonEmpty(values["LLMObservability.MaxOutputBytes"], values["LLMObs.MaxOutputBytes"]); strings.TrimSpace(value) != "" {
		maxOutputBytes, err := strconv.Atoi(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.MaxOutputBytes = maxOutputBytes
	}
	cfg.Langfuse = LangfuseObservabilityConfig{
		Host:      firstNonEmpty(values["LLMObservability.Langfuse.Host"], values["LLMObs.Langfuse.Host"], values["Langfuse.Host"]),
		PublicKey: firstNonEmpty(values["LLMObservability.Langfuse.PublicKey"], values["LLMObs.Langfuse.PublicKey"], values["Langfuse.PublicKey"]),
		SecretKey: firstNonEmpty(values["LLMObservability.Langfuse.SecretKey"], values["LLMObs.Langfuse.SecretKey"], values["Langfuse.SecretKey"]),
	}
	return ResolveLLMObservabilityConfig(cfg)
}

func pythonExecutorConfigFromValues(values map[string]string) (PythonExecutorConfig, error) {
	cfg := PythonExecutorConfig{
		Backend: firstNonEmpty(values["PythonExecutor.Backend"], values["PythonExecutorBackend"]),
		K8S: PythonExecutorK8SConfig{
			Namespace:          firstNonEmpty(values["PythonExecutor.K8S.Namespace"], values["PythonExecutorK8S.Namespace"], values["PythonExecutorK8SNamespace"]),
			Image:              firstNonEmpty(values["PythonExecutor.K8S.Image"], values["PythonExecutorK8S.Image"], values["PythonExecutorK8SImage"]),
			ServiceAccountName: firstNonEmpty(values["PythonExecutor.K8S.ServiceAccountName"], values["PythonExecutorK8S.ServiceAccountName"], values["PythonExecutorK8SServiceAccountName"]),
			RuntimeClassName:   firstNonEmpty(values["PythonExecutor.K8S.RuntimeClassName"], values["PythonExecutorK8S.RuntimeClassName"], values["PythonExecutorK8SRuntimeClassName"]),
		},
	}
	if value := firstNonEmpty(values["PythonExecutor.DefaultTimeoutSeconds"], values["PythonExecutorDefaultTimeoutSeconds"]); strings.TrimSpace(value) != "" {
		if expanded := strings.TrimSpace(os.ExpandEnv(value)); expanded != "" {
			seconds, err := strconv.Atoi(expanded)
			if err != nil {
				return cfg, err
			}
			cfg.DefaultTimeoutSeconds = seconds
		}
	}
	if value := firstNonEmpty(values["PythonExecutor.MaxTimeoutSeconds"], values["PythonExecutorMaxTimeoutSeconds"]); strings.TrimSpace(value) != "" {
		if expanded := strings.TrimSpace(os.ExpandEnv(value)); expanded != "" {
			seconds, err := strconv.Atoi(expanded)
			if err != nil {
				return cfg, err
			}
			cfg.MaxTimeoutSeconds = seconds
		}
	}
	if value := firstNonEmpty(values["PythonExecutor.DefaultMemoryMiB"], values["PythonExecutorDefaultMemoryMiB"]); strings.TrimSpace(value) != "" {
		if expanded := strings.TrimSpace(os.ExpandEnv(value)); expanded != "" {
			memoryMiB, err := strconv.Atoi(expanded)
			if err != nil {
				return cfg, err
			}
			cfg.DefaultMemoryMiB = memoryMiB
		}
	}
	if value := firstNonEmpty(values["PythonExecutor.MaxMemoryMiB"], values["PythonExecutorMaxMemoryMiB"]); strings.TrimSpace(value) != "" {
		if expanded := strings.TrimSpace(os.ExpandEnv(value)); expanded != "" {
			memoryMiB, err := strconv.Atoi(expanded)
			if err != nil {
				return cfg, err
			}
			cfg.MaxMemoryMiB = memoryMiB
		}
	}
	if value := firstNonEmpty(values["PythonExecutor.MaxOutputBytes"], values["PythonExecutorMaxOutputBytes"]); strings.TrimSpace(value) != "" {
		if expanded := strings.TrimSpace(os.ExpandEnv(value)); expanded != "" {
			maxOutputBytes, err := strconv.ParseInt(expanded, 10, 64)
			if err != nil {
				return cfg, err
			}
			cfg.MaxOutputBytes = maxOutputBytes
		}
	}
	return ResolvePythonExecutorConfig(cfg)
}

func objectStorageConfigFromValues(values map[string]string, storageDriver string) (ObjectStorageConfig, error) {
	cfg := ObjectStorageConfig{
		Driver:           firstNonEmpty(values["ObjectStorage.Driver"], values["ObjectStorageDriver"]),
		Endpoint:         firstNonEmpty(values["ObjectStorage.Endpoint"], values["ObjectStorageEndpoint"]),
		ExternalEndpoint: firstNonEmpty(values["ObjectStorage.ExternalEndpoint"], values["ObjectStorageExternalEndpoint"]),
		Bucket:           firstNonEmpty(values["ObjectStorage.Bucket"], values["ObjectStorageBucket"]),
		Region:           firstNonEmpty(values["ObjectStorage.Region"], values["ObjectStorageRegion"]),
		AccessKeyID:      firstNonEmpty(values["ObjectStorage.AccessKeyID"], values["ObjectStorageAccessKeyID"]),
		SecretAccessKey:  firstNonEmpty(values["ObjectStorage.SecretAccessKey"], values["ObjectStorageSecretAccessKey"]),
	}
	if value := firstNonEmpty(values["ObjectStorage.UseSSL"], values["ObjectStorageUseSSL"]); strings.TrimSpace(value) != "" {
		useSSL, err := strconv.ParseBool(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.UseSSL = useSSL
	}
	if value := firstNonEmpty(values["ObjectStorage.ExternalUseSSL"], values["ObjectStorageExternalUseSSL"]); strings.TrimSpace(value) != "" {
		externalUseSSL, err := parseOptionalBool(value)
		if err != nil {
			return cfg, err
		}
		cfg.ExternalUseSSL = externalUseSSL
	}
	return ResolveObjectStorageConfig(cfg, storageDriver)
}

func observabilityHTTPConfigFromValues(values map[string]string, current ObservabilityHTTPConfig) (ObservabilityHTTPConfig, error) {
	cfg := current
	if value := firstNonEmpty(values["Observability.Enabled"], values["ObservabilityHTTP.Enabled"]); strings.TrimSpace(value) != "" {
		enabled, err := strconv.ParseBool(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.Enabled = enabled
	}
	if value := firstNonEmpty(values["Observability.Host"], values["ObservabilityHTTP.Host"]); strings.TrimSpace(value) != "" {
		cfg.Host = strings.TrimSpace(os.ExpandEnv(value))
	}
	if value := firstNonEmpty(values["Observability.Port"], values["ObservabilityHTTP.Port"]); strings.TrimSpace(value) != "" {
		port, err := strconv.Atoi(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.Port = port
	}
	return ResolveObservabilityHTTPConfig(cfg)
}

func brokerListFromValue(value string) []string {
	value = strings.TrimSpace(os.ExpandEnv(value))
	if value == "" {
		return nil
	}
	rawBrokers := strings.Split(value, ",")
	brokers := make([]string, 0, len(rawBrokers))
	for _, broker := range rawBrokers {
		broker = strings.TrimSpace(broker)
		if broker != "" {
			brokers = append(brokers, broker)
		}
	}
	return brokers
}

func originListFromValue(value string) []string {
	value = strings.TrimSpace(os.ExpandEnv(value))
	if value == "" {
		return nil
	}
	rawOrigins := strings.Split(value, ",")
	origins := make([]string, 0, len(rawOrigins))
	for _, origin := range rawOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func normalizeOrigin(origin string) (string, error) {
	origin = strings.TrimSpace(os.ExpandEnv(origin))
	if origin == "" {
		return "", nil
	}
	if origin == "*" {
		return "", fmt.Errorf("gateway websocket allowed origin %q is invalid: wildcard origins are not allowed", origin)
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return "", fmt.Errorf("gateway websocket allowed origin %q is invalid: %w", origin, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("gateway websocket allowed origin %q is invalid: scheme must be http or https", origin)
	}
	if strings.TrimSpace(parsed.Host) == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("gateway websocket allowed origin %q is invalid: expected exact scheme://host[:port]", origin)
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), nil
}

func isBrowserLocalEndpoint(endpoint string) bool {
	host := endpointHost(endpoint)
	if host == "" {
		return false
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsUnspecified())
}

func endpointHost(endpoint string) string {
	endpoint = strings.TrimSpace(os.ExpandEnv(endpoint))
	if endpoint == "" {
		return ""
	}
	if strings.Contains(endpoint, "://") {
		parsed, err := url.Parse(endpoint)
		if err == nil {
			return strings.Trim(parsed.Hostname(), "[]")
		}
	}
	if slash := strings.Index(endpoint, "/"); slash >= 0 {
		endpoint = endpoint[:slash]
	}
	if host, _, err := net.SplitHostPort(endpoint); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(endpoint, "[]")
}

func resolveInt(current int, envValues ...string) (int, error) {
	if current != 0 {
		return current, nil
	}
	for _, value := range envValues {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		return strconv.Atoi(value)
	}
	return 0, nil
}

func resolveBool(current bool, envValues ...string) (bool, error) {
	for _, value := range envValues {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		return strconv.ParseBool(value)
	}
	return current, nil
}

func resolveOptionalBool(current *bool, fallback bool, envValues ...string) (*bool, error) {
	for _, value := range envValues {
		parsed, err := parseOptionalBool(value)
		if err != nil {
			return nil, err
		}
		if parsed != nil {
			return parsed, nil
		}
	}
	if current != nil {
		return current, nil
	}
	return boolPtr(fallback), nil
}

func parseOptionalBool(value string) (*bool, error) {
	value = strings.TrimSpace(os.ExpandEnv(value))
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func boolPtr(value bool) *bool {
	return &value
}

func resolveInt64(current int64, envValues ...string) (int64, error) {
	if current != 0 {
		return current, nil
	}
	for _, value := range envValues {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		return strconv.ParseInt(value, 10, 64)
	}
	return 0, nil
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func readFlatYAML(path string) (map[string]string, error) {
	values := make(map[string]string)
	if strings.TrimSpace(path) == "" {
		return values, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return values, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return values, err
	}
	if len(doc.Content) == 0 {
		return values, nil
	}

	flattenYAML(values, "", doc.Content[0])
	return values, nil
}

func flattenYAML(values map[string]string, prefix string, node *yaml.Node) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := strings.TrimSpace(node.Content[i].Value)
			if key == "" {
				continue
			}
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			flattenYAML(values, next, node.Content[i+1])
		}
	case yaml.SequenceNode:
		items := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			if item.Kind != yaml.ScalarNode {
				continue
			}
			value := strings.TrimSpace(item.Value)
			if value != "" {
				items = append(items, value)
			}
		}
		if prefix != "" {
			values[prefix] = strings.Join(items, ",")
		}
	case yaml.ScalarNode:
		if prefix != "" {
			values[prefix] = strings.TrimSpace(node.Value)
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func defaultWorkerID() string {
	if hostname := strings.TrimSpace(os.Getenv("HOSTNAME")); hostname != "" {
		return hostname
	}
	return "msgtransfer-local"
}
