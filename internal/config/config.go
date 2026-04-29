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
}

type RPCConfig struct {
	Name          string
	ListenOn      string
	Auth          JWTAuthConfig
	StorageDriver string
	DataSource    string
	Redis         RedisConfig
	Presence      PresenceConfig
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

const (
	StorageDriverMemory   = "memory"
	StorageDriverPostgres = "postgres"
	PresenceDriverMemory  = "memory"
	PresenceDriverRedis   = "redis"
)

const (
	defaultRedisAddr                   = "localhost:6379"
	defaultPresenceHeartbeatTTLSeconds = 60
	defaultPresenceRedisKeyPrefix      = "agents_im:presence"
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
