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
	"github.com/zeromicro/go-zero/zrpc"
	"gopkg.in/yaml.v3"
)

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
	StorageDriverMemory       = "memory"
	StorageDriverPostgres     = "postgres"
	ObjectStorageDriverMemory = "memory"
	// ObjectStorageDriverRustFS 是 S3 兼容对象存储（RustFS，替代退役的 MinIO，见 #569）。旧值
	// "minio"/"s3" 作 legacy 别名仍被接受、归一化到本值（见 ResolveObjectStorageConfig），使存量
	// secret/配置在切换期不中断。
	ObjectStorageDriverRustFS     = "rustfs"
	PresenceDriverMemory          = "memory"
	PresenceDriverRedis           = "redis"
	TransferConsumerMemory        = "memory"
	TransferConsumerOutbox        = "outbox"
	TransferDispatcherNoop        = "noop"
	TransferDispatcherGateway     = "gateway"
	PythonExecutorBackendDisabled = "disabled"
	PythonExecutorBackendK8S      = "k8s"
)

const (
	defaultRedisAddr                   = "localhost:6379"
	defaultPresenceHeartbeatTTLSeconds = 60
	defaultPresenceRedisKeyPrefix      = "agents_im:presence"
	defaultObjectStorageBucket         = "agents-im-media"
	defaultObjectStorageRegion         = "us-east-1"
	defaultTransferTopic               = "message.events.v1"
	defaultTransferGroup               = "msgtransfer-worker"
	defaultTransferPollIntervalMillis  = 100
	defaultTransferRetryBackoffMillis  = 1000
	defaultTransferMaxAttempts         = 5
	defaultTransferObservabilityHost   = "0.0.0.0"
	defaultTransferObservabilityPort   = 8085
	defaultPythonExecutorTimeout       = 10
	defaultPythonExecutorMaxTimeout    = 30
	defaultPythonExecutorMemoryMiB     = 256
	defaultPythonExecutorMaxOutput     = 64 * 1024
)

var ErrObjectStorageExternalEndpointLoopback = errors.New("object storage external endpoint cannot be loopback in production")

func DefaultJWTAuthConfig() JWTAuthConfig {
	return JWTAuthConfig{
		AccessSecret: "dev-jwt-secret-change-me",
		AccessExpire: 86400,
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

func LoadMessageTransferConfig(path string) (MessageTransferConfig, error) {
	cfg := DefaultMessageTransferConfig()
	values, err := readFlatYAML(path)
	if err != nil {
		return cfg, err
	}

	if value := values["Name"]; value != "" {
		cfg.Name = value
	}
	cfg.WorkerID = FirstNonEmpty(values["Worker.ID"], values["WorkerID"], os.Getenv("MESSAGE_TRANSFER_WORKER_ID"), cfg.WorkerID)
	if value := FirstNonEmpty(values["StorageDriver"], values["Repository"]); value != "" {
		cfg.StorageDriver = ResolveStorageDriver(value)
	} else {
		cfg.StorageDriver = ResolveStorageDriver(cfg.StorageDriver)
	}
	cfg.DataSource = ResolveDataSource(values["DataSource"])
	cfg.Tracing, err = tracingConfigFromValues(values, cfg.Tracing, cfg.Name)
	if err != nil {
		return cfg, err
	}
	if value := FirstNonEmpty(values["Consumer.Driver"], values["ConsumerDriver"]); value != "" {
		cfg.Consumer.Driver = ResolveTransferConsumerDriver(value)
	} else {
		cfg.Consumer.Driver = ResolveTransferConsumerDriver(cfg.Consumer.Driver)
	}
	if value := FirstNonEmpty(values["Consumer.Topic"], values["Topic"]); value != "" {
		cfg.Consumer.Topic = strings.TrimSpace(os.ExpandEnv(value))
	} else {
		cfg.Consumer.Topic = ""
	}
	if value := FirstNonEmpty(values["Consumer.Group"], values["ConsumerGroup"]); value != "" {
		cfg.Consumer.Group = strings.TrimSpace(os.ExpandEnv(value))
	} else {
		cfg.Consumer.Group = ""
	}
	if value := FirstNonEmpty(values["Dispatcher.Driver"], values["DispatcherDriver"]); value != "" {
		cfg.Dispatcher.Driver = ResolveTransferDispatcherDriver(value)
	} else {
		cfg.Dispatcher.Driver = ResolveTransferDispatcherDriver(cfg.Dispatcher.Driver)
	}
	cfg.Dispatcher.GatewayEndpoint = FirstNonEmpty(
		strings.TrimSpace(os.ExpandEnv(values["Dispatcher.GatewayEndpoint"])),
		strings.TrimSpace(os.ExpandEnv(values["Dispatcher.Endpoint"])),
		os.Getenv("MESSAGE_TRANSFER_GATEWAY_ENDPOINT"),
		cfg.Dispatcher.GatewayEndpoint,
	)
	if value := FirstNonEmpty(values["Worker.PollIntervalMillis"], values["PollIntervalMillis"]); value != "" {
		interval, err := strconv.ParseInt(strings.TrimSpace(os.ExpandEnv(value)), 10, 64)
		if err != nil {
			return cfg, err
		}
		cfg.Worker.PollIntervalMillis = interval
	}
	if value := FirstNonEmpty(values["Worker.RetryBackoffMillis"], values["RetryBackoffMillis"]); value != "" {
		backoff, err := strconv.ParseInt(strings.TrimSpace(os.ExpandEnv(value)), 10, 64)
		if err != nil {
			return cfg, err
		}
		cfg.Worker.RetryBackoffMillis = backoff
	}
	if value := FirstNonEmpty(values["Worker.MaxAttempts"], values["MaxAttempts"]); value != "" {
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
	cfg.Kafka.Brokers = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(values["Kafka.Brokers"])), cfg.Kafka.Brokers)
	if value := values["Kafka.Workers"]; value != "" {
		workers, err := strconv.Atoi(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.Kafka.Workers = workers
	}
	cfg.Kafka.Redis.Addr = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(values["Kafka.Redis.Addr"])), cfg.Kafka.Redis.Addr)
	cfg.Kafka.Redis.Password = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(values["Kafka.Redis.Password"])), cfg.Kafka.Redis.Password)
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
	cfg.Name = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Name)), "msgtransfer")
	cfg.WorkerID = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.WorkerID)), os.Getenv("MESSAGE_TRANSFER_WORKER_ID"), defaultWorkerID())
	cfg.StorageDriver = ResolveStorageDriver(cfg.StorageDriver)
	cfg.DataSource = ResolveDataSource(cfg.DataSource)
	if tracing, err := observability.ResolveTracingConfig(cfg.Tracing, cfg.Name); err == nil {
		cfg.Tracing = tracing
	}
	cfg.Consumer.Driver = ResolveTransferConsumerDriver(cfg.Consumer.Driver)
	cfg.Consumer.Topic = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Consumer.Topic)), os.Getenv("MESSAGE_TRANSFER_TOPIC"), defaultTransferTopic)
	cfg.Consumer.Group = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Consumer.Group)), os.Getenv("MESSAGE_TRANSFER_CONSUMER_GROUP"), defaultTransferGroup)
	cfg.Dispatcher.Driver = ResolveTransferDispatcherDriver(cfg.Dispatcher.Driver)
	cfg.Dispatcher.GatewayEndpoint = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Dispatcher.GatewayEndpoint)), os.Getenv("MESSAGE_TRANSFER_GATEWAY_ENDPOINT"))
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
	cfg.Brokers = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Brokers)), os.Getenv("KAFKA_BROKERS"))
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
	if value := FirstNonEmpty(os.Getenv("MESSAGE_TRANSFER_OBSERVABILITY_ENABLED"), os.Getenv("AGENTS_IM_OBSERVABILITY_ENABLED")); value != "" {
		enabled, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return cfg, err
		}
		cfg.Enabled = enabled
	}
	cfg.Host = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Host)), os.Getenv("MESSAGE_TRANSFER_OBSERVABILITY_HOST"), defaultTransferObservabilityHost)
	port, err := ResolveInt(cfg.Port, os.Getenv("MESSAGE_TRANSFER_OBSERVABILITY_PORT"))
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
	cfg.Addr = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Addr)), os.Getenv("REDIS_ADDR"), os.Getenv("AGENTS_IM_REDIS_ADDR"), defaultRedisAddr)
	cfg.Password = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Password)), os.Getenv("REDIS_PASSWORD"), os.Getenv("AGENTS_IM_REDIS_PASSWORD"))

	db, err := ResolveInt(cfg.DB, os.Getenv("REDIS_DB"), os.Getenv("AGENTS_IM_REDIS_DB"))
	if err != nil {
		return cfg, err
	}
	cfg.DB = db
	return cfg, nil
}

func ResolvePresenceConfig(cfg PresenceConfig) (PresenceConfig, error) {
	cfg.Driver = ResolvePresenceDriver(cfg.Driver)

	ttl, err := ResolveInt64(cfg.HeartbeatTTLSeconds, os.Getenv("PRESENCE_TTL_SECONDS"), os.Getenv("AGENTS_IM_PRESENCE_TTL_SECONDS"))
	if err != nil {
		return cfg, err
	}
	if ttl <= 0 {
		ttl = defaultPresenceHeartbeatTTLSeconds
	}
	cfg.HeartbeatTTLSeconds = ttl
	cfg.KeyPrefix = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.KeyPrefix)), os.Getenv("PRESENCE_KEY_PREFIX"), os.Getenv("AGENTS_IM_PRESENCE_KEY_PREFIX"), defaultPresenceRedisKeyPrefix)
	return cfg, nil
}

func ResolvePythonExecutorConfig(cfg PythonExecutorConfig) (PythonExecutorConfig, error) {
	defaults := DefaultPythonExecutorConfig()
	backend := strings.ToLower(strings.TrimSpace(os.ExpandEnv(cfg.Backend)))
	if backend == "" {
		backend = strings.ToLower(strings.TrimSpace(FirstNonEmpty(os.Getenv("PYTHON_EXECUTOR_BACKEND"), os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_BACKEND"))))
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

	cfg.K8S.Namespace = FirstNonEmpty(
		strings.TrimSpace(os.ExpandEnv(cfg.K8S.Namespace)),
		os.Getenv("PYTHON_EXECUTOR_K8S_NAMESPACE"),
		os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_K8S_NAMESPACE"),
	)
	cfg.K8S.Image = FirstNonEmpty(
		strings.TrimSpace(os.ExpandEnv(cfg.K8S.Image)),
		os.Getenv("PYTHON_EXECUTOR_K8S_IMAGE"),
		os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_K8S_IMAGE"),
	)
	cfg.K8S.ServiceAccountName = FirstNonEmpty(
		strings.TrimSpace(os.ExpandEnv(cfg.K8S.ServiceAccountName)),
		os.Getenv("PYTHON_EXECUTOR_K8S_SERVICE_ACCOUNT_NAME"),
		os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_K8S_SERVICE_ACCOUNT_NAME"),
	)
	cfg.K8S.RuntimeClassName = FirstNonEmpty(
		strings.TrimSpace(os.ExpandEnv(cfg.K8S.RuntimeClassName)),
		os.Getenv("PYTHON_EXECUTOR_K8S_RUNTIME_CLASS_NAME"),
		os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_K8S_RUNTIME_CLASS_NAME"),
	)

	defaultTimeout, err := ResolveInt(cfg.DefaultTimeoutSeconds, os.Getenv("PYTHON_EXECUTOR_DEFAULT_TIMEOUT_SECONDS"), os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_DEFAULT_TIMEOUT_SECONDS"))
	if err != nil {
		return cfg, err
	}
	if defaultTimeout <= 0 {
		defaultTimeout = defaults.DefaultTimeoutSeconds
	}
	cfg.DefaultTimeoutSeconds = defaultTimeout

	maxTimeout, err := ResolveInt(cfg.MaxTimeoutSeconds, os.Getenv("PYTHON_EXECUTOR_MAX_TIMEOUT_SECONDS"), os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_MAX_TIMEOUT_SECONDS"))
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

	defaultMemoryMiB, err := ResolveInt(cfg.DefaultMemoryMiB, os.Getenv("PYTHON_EXECUTOR_DEFAULT_MEMORY_MIB"), os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_DEFAULT_MEMORY_MIB"))
	if err != nil {
		return cfg, err
	}
	if defaultMemoryMiB <= 0 {
		defaultMemoryMiB = defaults.DefaultMemoryMiB
	}
	cfg.DefaultMemoryMiB = defaultMemoryMiB

	maxMemoryMiB, err := ResolveInt(cfg.MaxMemoryMiB, os.Getenv("PYTHON_EXECUTOR_MAX_MEMORY_MIB"), os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_MAX_MEMORY_MIB"))
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

	maxOutputBytes, err := ResolveInt64(cfg.MaxOutputBytes, os.Getenv("PYTHON_EXECUTOR_MAX_OUTPUT_BYTES"), os.Getenv("AGENTS_IM_PYTHON_EXECUTOR_MAX_OUTPUT_BYTES"))
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

func ResolveObjectStorageConfig(cfg ObjectStorageConfig, storageDriver string) (ObjectStorageConfig, error) {
	driver := strings.ToLower(strings.TrimSpace(os.ExpandEnv(cfg.Driver)))
	if driver == "" {
		driver = strings.ToLower(strings.TrimSpace(FirstNonEmpty(os.Getenv("OBJECT_STORAGE_DRIVER"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_DRIVER"))))
	}
	if driver == "" {
		if ResolveStorageDriver(storageDriver) == StorageDriverMemory {
			driver = ObjectStorageDriverMemory
		} else {
			driver = ObjectStorageDriverRustFS
		}
	}
	switch driver {
	case ObjectStorageDriverMemory:
		cfg.Driver = ObjectStorageDriverMemory
	case ObjectStorageDriverRustFS, "minio", "s3":
		// "minio"/"s3" 是 legacy 别名（MinIO 退役前的存量值），归一化到 rustfs（同走 S3 客户端）。
		cfg.Driver = ObjectStorageDriverRustFS
	default:
		return cfg, fmt.Errorf("unsupported object storage driver %q; use %q for dev/test memory mode or %q for S3-compatible storage (RustFS)", driver, ObjectStorageDriverMemory, ObjectStorageDriverRustFS)
	}

	cfg.Endpoint = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Endpoint)), os.Getenv("OBJECT_STORAGE_ENDPOINT"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_ENDPOINT"))
	cfg.ExternalEndpoint = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.ExternalEndpoint)), os.Getenv("OBJECT_STORAGE_EXTERNAL_ENDPOINT"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_EXTERNAL_ENDPOINT"))
	cfg.Bucket = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Bucket)), os.Getenv("OBJECT_STORAGE_BUCKET"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_BUCKET"), defaultObjectStorageBucket)
	cfg.Region = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Region)), os.Getenv("OBJECT_STORAGE_REGION"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_REGION"), defaultObjectStorageRegion)
	cfg.AccessKeyID = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.AccessKeyID)), os.Getenv("OBJECT_STORAGE_ACCESS_KEY_ID"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_ACCESS_KEY_ID"), os.Getenv("OSS_ROOT_USER"), os.Getenv("MINIO_ROOT_USER"))
	cfg.SecretAccessKey = FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.SecretAccessKey)), os.Getenv("OBJECT_STORAGE_SECRET_ACCESS_KEY"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_SECRET_ACCESS_KEY"), os.Getenv("OSS_ROOT_PASSWORD"), os.Getenv("MINIO_ROOT_PASSWORD"))

	useSSL, err := ResolveBool(cfg.UseSSL, os.Getenv("OBJECT_STORAGE_USE_SSL"), os.Getenv("AGENTS_IM_OBJECT_STORAGE_USE_SSL"))
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
	if ResolveStorageDriver(storageDriver) != StorageDriverPostgres || cfg.Driver != ObjectStorageDriverRustFS {
		return nil
	}
	presignEndpoint := FirstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.ExternalEndpoint)), strings.TrimSpace(os.ExpandEnv(cfg.Endpoint)))
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

func tracingConfigFromValues(values map[string]string, current observability.TracingConfig, serviceName string) (observability.TracingConfig, error) {
	cfg := current
	if value := FirstNonEmpty(values["Tracing.Enabled"], values["TracingEnabled"]); strings.TrimSpace(value) != "" {
		enabled, err := strconv.ParseBool(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.Enabled = enabled
	}
	cfg.ServiceName = FirstNonEmpty(values["Tracing.ServiceName"], values["TracingServiceName"], cfg.ServiceName)
	cfg.Environment = FirstNonEmpty(values["Tracing.Environment"], values["TracingEnv"], cfg.Environment)
	cfg.OTLPEndpoint = FirstNonEmpty(values["Tracing.OTLPEndpoint"], values["Tracing.Endpoint"], values["OTLPEndpoint"], cfg.OTLPEndpoint)
	cfg.Protocol = FirstNonEmpty(values["Tracing.Protocol"], values["OTLPProtocol"], cfg.Protocol)
	if value := FirstNonEmpty(values["Tracing.SamplerRatio"], values["TracingSamplerRatio"]); strings.TrimSpace(value) != "" {
		ratio, err := strconv.ParseFloat(strings.TrimSpace(os.ExpandEnv(value)), 64)
		if err != nil {
			return cfg, err
		}
		cfg.SamplerRatio = ratio
	}
	cfg.TraceUIBaseURL = FirstNonEmpty(values["Tracing.TraceUIBaseURL"], values["TraceUIBaseURL"], cfg.TraceUIBaseURL)
	if value := FirstNonEmpty(values["Tracing.Insecure"], values["TracingInsecure"]); strings.TrimSpace(value) != "" {
		insecure, err := strconv.ParseBool(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.Insecure = insecure
	}
	return observability.ResolveTracingConfig(cfg, serviceName)
}

// rpcClientConfigSpec 描述某个 RPC 客户端配置的差异点：YAML key 前缀、env 兜底 key、
// 默认 timeout。mail/msg/user/agent 四组 zrpc 客户端配置结构一致，仅这些参数不同，
// 共用 resolveRPCClientConfig，避免逐域复制粘贴。
type rpcClientConfigSpec struct {
	prefix          string
	targetEnvKeys   []string
	endpointEnvKeys []string
	defaultTimeout  int64
}

var (
	mailRPCSpec = rpcClientConfigSpec{
		prefix:          "MailRPC",
		targetEnvKeys:   []string{"AUTH_MAIL_RPC_TARGET", "AGENTS_IM_MAIL_RPC_TARGET", "MAIL_RPC_TARGET"},
		endpointEnvKeys: []string{"AUTH_MAIL_RPC_ENDPOINTS", "AGENTS_IM_MAIL_RPC_ENDPOINTS", "MAIL_RPC_ENDPOINTS"},
		defaultTimeout:  2000,
	}
	msgRPCSpec = rpcClientConfigSpec{
		prefix:          "MsgRPC",
		targetEnvKeys:   []string{"AGENTS_IM_MSG_RPC_TARGET", "MSG_RPC_TARGET"},
		endpointEnvKeys: []string{"AGENTS_IM_MSG_RPC_ENDPOINTS", "MSG_RPC_ENDPOINTS"},
		defaultTimeout:  5000,
	}
	userRPCSpec = rpcClientConfigSpec{
		prefix:          "UserRPC",
		targetEnvKeys:   []string{"AGENTS_IM_USER_RPC_TARGET", "USER_RPC_TARGET"},
		endpointEnvKeys: []string{"AGENTS_IM_USER_RPC_ENDPOINTS", "USER_RPC_ENDPOINTS"},
		defaultTimeout:  5000,
	}
	agentRPCSpec = rpcClientConfigSpec{
		prefix:          "AgentRPC",
		targetEnvKeys:   []string{"AGENTS_IM_AGENT_RPC_TARGET", "AGENT_RPC_TARGET"},
		endpointEnvKeys: []string{"AGENTS_IM_AGENT_RPC_ENDPOINTS", "AGENT_RPC_ENDPOINTS"},
		defaultTimeout:  5000,
	}
)

// resolveRPCClientConfig 套用 spec 的 env 兜底（target / endpoints）、默认 timeout 与 NonBlock。
// YAML 取到的 Target 经 ExpandEnv 后优先，其次按 spec.targetEnvKeys 顺序兜底。
func resolveRPCClientConfig(cfg zrpc.RpcClientConf, spec rpcClientConfigSpec) zrpc.RpcClientConf {
	targetCandidates := make([]string, 0, len(spec.targetEnvKeys)+1)
	targetCandidates = append(targetCandidates, strings.TrimSpace(os.ExpandEnv(cfg.Target)))
	for _, key := range spec.targetEnvKeys {
		targetCandidates = append(targetCandidates, os.Getenv(key))
	}
	cfg.Target = FirstNonEmpty(targetCandidates...)
	if len(cfg.Endpoints) == 0 {
		endpointCandidates := make([]string, 0, len(spec.endpointEnvKeys))
		for _, key := range spec.endpointEnvKeys {
			endpointCandidates = append(endpointCandidates, os.Getenv(key))
		}
		cfg.Endpoints = brokerListFromValue(FirstNonEmpty(endpointCandidates...))
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = spec.defaultTimeout
	}
	cfg.NonBlock = true
	return cfg
}

// 以下 exported wrapper 保留稳定符号供外部调用与契约守卫使用，委托到 resolveRPCClientConfig。

// ResolveMailRPCConfig 解析邮件 third-rpc 客户端配置；env 兜底 AUTH_MAIL_RPC_TARGET /
// AGENTS_IM_MAIL_RPC_TARGET / MAIL_RPC_TARGET。
func ResolveMailRPCConfig(cfg zrpc.RpcClientConf) zrpc.RpcClientConf {
	return resolveRPCClientConfig(cfg, mailRPCSpec)
}

// ResolveMsgRPCConfig 解析 msggateway → msg-rpc 客户端配置；env 兜底 MSG_RPC_TARGET /
// AGENTS_IM_MSG_RPC_TARGET。
func ResolveMsgRPCConfig(cfg zrpc.RpcClientConf) zrpc.RpcClientConf {
	return resolveRPCClientConfig(cfg, msgRPCSpec)
}

// ResolveUserRPCConfig 解析读用户资料的 user-rpc 客户端配置；env 兜底 USER_RPC_TARGET /
// AGENTS_IM_USER_RPC_TARGET。
func ResolveUserRPCConfig(cfg zrpc.RpcClientConf) zrpc.RpcClientConf {
	return resolveRPCClientConfig(cfg, userRPCSpec)
}

// ResolveAgentRPCConfig 解析 agent-api（纯 BFF，#606）→ 属主 agent-rpc 客户端配置；env 兜底
// AGENT_RPC_TARGET / AGENTS_IM_AGENT_RPC_TARGET。
func ResolveAgentRPCConfig(cfg zrpc.RpcClientConf) zrpc.RpcClientConf {
	return resolveRPCClientConfig(cfg, agentRPCSpec)
}

func observabilityHTTPConfigFromValues(values map[string]string, current ObservabilityHTTPConfig) (ObservabilityHTTPConfig, error) {
	cfg := current
	if value := FirstNonEmpty(values["Observability.Enabled"], values["ObservabilityHTTP.Enabled"]); strings.TrimSpace(value) != "" {
		enabled, err := strconv.ParseBool(strings.TrimSpace(os.ExpandEnv(value)))
		if err != nil {
			return cfg, err
		}
		cfg.Enabled = enabled
	}
	if value := FirstNonEmpty(values["Observability.Host"], values["ObservabilityHTTP.Host"]); strings.TrimSpace(value) != "" {
		cfg.Host = strings.TrimSpace(os.ExpandEnv(value))
	}
	if value := FirstNonEmpty(values["Observability.Port"], values["ObservabilityHTTP.Port"]); strings.TrimSpace(value) != "" {
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

// ResolveInt returns current when non-zero, else the first parseable env value.
// Exported so domain config packages moved out of pkg/config can reuse the shared
// env-override parsing instead of duplicating it.
func ResolveInt(current int, envValues ...string) (int, error) {
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

// ResolveBool returns the first parseable env value, else current. Exported for
// reuse by domain config packages (see ResolveInt).
func ResolveBool(current bool, envValues ...string) (bool, error) {
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

// ResolveInt64 returns current when non-zero, else the first parseable env value.
// Exported for reuse by domain config packages (see ResolveInt).
func ResolveInt64(current int64, envValues ...string) (int64, error) {
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

// FirstNonEmpty returns the first value that is non-empty after trimming.
// Exported for reuse by domain config packages (see ResolveInt).
func FirstNonEmpty(values ...string) string {
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
