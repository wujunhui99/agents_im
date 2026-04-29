package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/rest"
)

type APIConfig struct {
	Name          string
	Host          string
	Port          int
	Auth          JWTAuthConfig
	StorageDriver string
	DataSource    string
	Redis         RedisConfig
	Presence      PresenceConfig
	Kafka         KafkaConfig
}

type RPCConfig struct {
	Name          string
	ListenOn      string
	Auth          JWTAuthConfig
	StorageDriver string
	DataSource    string
	Redis         RedisConfig
	Presence      PresenceConfig
	Kafka         KafkaConfig
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

type KafkaConfig struct {
	Brokers            []string
	MessageEventsTopic string
	ConsumerGroup      string
}

type MessageTransferConfig struct {
	Name          string
	WorkerID      string
	DryRun        bool
	StorageDriver string
	DataSource    string
	Kafka         KafkaConfig
	Consumer      TransferConsumerConfig
	Dispatcher    TransferDispatcherConfig
	Worker        TransferWorkerConfig
	Observability ObservabilityHTTPConfig
}

type TransferConsumerConfig struct {
	Driver string
	Topic  string
	Group  string
}

type TransferDispatcherConfig struct {
	Driver string
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
	StorageDriverMemory    = "memory"
	StorageDriverPostgres  = "postgres"
	PresenceDriverMemory   = "memory"
	PresenceDriverRedis    = "redis"
	TransferConsumerMemory = "memory"
	TransferConsumerKafka  = "kafka"
	TransferDispatcherNoop = "noop"
)

const (
	defaultRedisAddr                   = "localhost:6379"
	defaultPresenceHeartbeatTTLSeconds = 60
	defaultPresenceRedisKeyPrefix      = "agents_im:presence"
	defaultKafkaBroker                 = "localhost:19092"
	defaultKafkaMessageEventsTopic     = "message.events.v1"
	defaultKafkaConsumerGroup          = "message-transfer-worker"
	defaultTransferTopic               = defaultKafkaMessageEventsTopic
	defaultTransferGroup               = defaultKafkaConsumerGroup
	defaultTransferPollIntervalMillis  = 100
	defaultTransferRetryBackoffMillis  = 1000
	defaultTransferMaxAttempts         = 5
	defaultTransferObservabilityHost   = "0.0.0.0"
	defaultTransferObservabilityPort   = 8085
)

func DefaultAPIConfig() APIConfig {
	return APIConfig{
		Name:          "user-api",
		Host:          "0.0.0.0",
		Port:          8080,
		Auth:          DefaultJWTAuthConfig(),
		StorageDriver: StorageDriverMemory,
		Redis:         DefaultRedisConfig(),
		Presence:      DefaultPresenceConfig(),
		Kafka:         DefaultKafkaConfig(),
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
		Kafka:         DefaultKafkaConfig(),
	}
}

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

func DefaultKafkaConfig() KafkaConfig {
	return KafkaConfig{
		Brokers:            []string{defaultKafkaBroker},
		MessageEventsTopic: defaultKafkaMessageEventsTopic,
		ConsumerGroup:      defaultKafkaConsumerGroup,
	}
}

func DefaultMessageTransferConfig() MessageTransferConfig {
	return MessageTransferConfig{
		Name:          "message-transfer",
		WorkerID:      defaultWorkerID(),
		DryRun:        true,
		StorageDriver: StorageDriverMemory,
		Kafka:         DefaultKafkaConfig(),
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
	if value := values["Auth.AccessSecret"]; value != "" {
		cfg.Auth.AccessSecret = value
	}
	if value := values["Auth.AccessExpire"]; value != "" {
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
	cfg.Kafka = kafkaConfigFromValues(values)

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
	if value := values["Auth.AccessSecret"]; value != "" {
		cfg.Auth.AccessSecret = value
	}
	if value := values["Auth.AccessExpire"]; value != "" {
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
	cfg.Kafka = kafkaConfigFromValues(values)

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
	cfg.Kafka = kafkaConfigFromValues(values)
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
	cfg = ResolveMessageTransferConfig(cfg)
	return cfg, nil
}

func ToRestConf(cfg APIConfig) rest.RestConf {
	var restConf rest.RestConf
	restConf.Name = cfg.Name
	restConf.Host = cfg.Host
	restConf.Port = cfg.Port
	return restConf
}

func ResolveStorageDriver(value string) string {
	value = strings.ToLower(strings.TrimSpace(os.ExpandEnv(value)))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(os.Getenv("AGENTS_IM_STORAGE_DRIVER")))
	}
	switch value {
	case StorageDriverPostgres:
		return StorageDriverPostgres
	default:
		return StorageDriverMemory
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
	case TransferConsumerKafka, "redpanda":
		return TransferConsumerKafka
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
	default:
		return TransferDispatcherNoop
	}
}

func ResolveMessageTransferConfig(cfg MessageTransferConfig) MessageTransferConfig {
	cfg.Name = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Name)), "message-transfer")
	cfg.WorkerID = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.WorkerID)), os.Getenv("MESSAGE_TRANSFER_WORKER_ID"), defaultWorkerID())
	cfg.StorageDriver = ResolveStorageDriver(cfg.StorageDriver)
	cfg.DataSource = ResolveDataSource(cfg.DataSource)
	cfg.Kafka = ResolveKafkaConfig(cfg.Kafka)
	cfg.Consumer.Driver = ResolveTransferConsumerDriver(cfg.Consumer.Driver)
	cfg.Consumer.Topic = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Consumer.Topic)), os.Getenv("MESSAGE_TRANSFER_TOPIC"), cfg.Kafka.MessageEventsTopic, defaultTransferTopic)
	cfg.Consumer.Group = firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.Consumer.Group)), os.Getenv("MESSAGE_TRANSFER_CONSUMER_GROUP"), cfg.Kafka.ConsumerGroup, defaultTransferGroup)
	cfg.Dispatcher.Driver = ResolveTransferDispatcherDriver(cfg.Dispatcher.Driver)
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
	return cfg
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

func ResolveKafkaConfig(cfg KafkaConfig) KafkaConfig {
	brokers := brokerListFromValue(strings.Join(cfg.Brokers, ","))
	if len(brokers) == 0 {
		brokers = brokerListFromValue(firstNonEmpty(os.Getenv("KAFKA_BROKERS"), os.Getenv("AGENTS_IM_KAFKA_BROKERS")))
	}
	if len(brokers) == 0 {
		brokers = []string{defaultKafkaBroker}
	}

	return KafkaConfig{
		Brokers:            brokers,
		MessageEventsTopic: firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.MessageEventsTopic)), os.Getenv("KAFKA_MESSAGE_EVENTS_TOPIC"), os.Getenv("AGENTS_IM_KAFKA_MESSAGE_EVENTS_TOPIC"), defaultKafkaMessageEventsTopic),
		ConsumerGroup:      firstNonEmpty(strings.TrimSpace(os.ExpandEnv(cfg.ConsumerGroup)), os.Getenv("KAFKA_CONSUMER_GROUP"), os.Getenv("AGENTS_IM_KAFKA_CONSUMER_GROUP"), defaultKafkaConsumerGroup),
	}
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

func kafkaConfigFromValues(values map[string]string) KafkaConfig {
	cfg := KafkaConfig{
		Brokers:            brokerListFromValue(firstNonEmpty(values["Kafka.Brokers"], values["KafkaBrokers"])),
		MessageEventsTopic: firstNonEmpty(values["Kafka.MessageEventsTopic"], values["KafkaMessageEventsTopic"]),
		ConsumerGroup:      firstNonEmpty(values["Kafka.ConsumerGroup"], values["KafkaConsumerGroup"]),
	}
	return ResolveKafkaConfig(cfg)
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

func readFlatYAML(path string) (map[string]string, error) {
	values := make(map[string]string)
	if strings.TrimSpace(path) == "" {
		return values, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return values, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	section := ""
	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if !strings.HasPrefix(rawLine, " ") && !strings.HasPrefix(rawLine, "\t") {
			if value == "" {
				section = key
				continue
			}
			section = ""
		}
		if section != "" && (strings.HasPrefix(rawLine, " ") || strings.HasPrefix(rawLine, "\t")) {
			key = section + "." + key
		}
		values[key] = value
	}

	return values, scanner.Err()
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
	return "message-transfer-local"
}
