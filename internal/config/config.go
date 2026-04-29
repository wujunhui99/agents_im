package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/rest"
)

type APIConfig struct {
	Name string
	Host string
	Port int
	Auth JWTAuthConfig
}

type RPCConfig struct {
	Name     string
	ListenOn string
}

type JWTAuthConfig struct {
	AccessSecret string
	AccessExpire int64
}

func DefaultAPIConfig() APIConfig {
	return APIConfig{Name: "user-api", Host: "0.0.0.0", Port: 8080, Auth: DefaultJWTAuthConfig()}
}

func DefaultRPCConfig() RPCConfig {
	return RPCConfig{Name: "user-rpc", ListenOn: "0.0.0.0:9090"}
}

func DefaultJWTAuthConfig() JWTAuthConfig {
	return JWTAuthConfig{
		AccessSecret: "dev-jwt-secret-change-me",
		AccessExpire: 86400,
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

	return cfg, nil
}

func ToRestConf(cfg APIConfig) rest.RestConf {
	var restConf rest.RestConf
	restConf.Name = cfg.Name
	restConf.Host = cfg.Host
	restConf.Port = cfg.Port
	return restConf
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
