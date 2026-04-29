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
	StorageDriver string
	DataSource    string
}

type RPCConfig struct {
	Name          string
	ListenOn      string
	StorageDriver string
	DataSource    string
}

const (
	StorageDriverMemory   = "memory"
	StorageDriverPostgres = "postgres"
)

func DefaultAPIConfig() APIConfig {
	return APIConfig{Name: "user-api", Host: "0.0.0.0", Port: 8080, StorageDriver: StorageDriverMemory}
}

func DefaultRPCConfig() RPCConfig {
	return RPCConfig{Name: "user-rpc", ListenOn: "0.0.0.0:9090", StorageDriver: StorageDriverMemory}
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
	if value := firstNonEmpty(values["StorageDriver"], values["Repository"]); value != "" {
		cfg.StorageDriver = ResolveStorageDriver(value)
	} else {
		cfg.StorageDriver = ResolveStorageDriver(cfg.StorageDriver)
	}
	cfg.DataSource = ResolveDataSource(values["DataSource"])

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
	if value := firstNonEmpty(values["StorageDriver"], values["Repository"]); value != "" {
		cfg.StorageDriver = ResolveStorageDriver(value)
	} else {
		cfg.StorageDriver = ResolveStorageDriver(cfg.StorageDriver)
	}
	cfg.DataSource = ResolveDataSource(values["DataSource"])

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
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
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
