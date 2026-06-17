package objectstorage

import (
	"fmt"

	appconfig "github.com/wujunhui99/agents_im/pkg/config"
)

func NewStore(cfg appconfig.ObjectStorageConfig) (ObjectStore, error) {
	switch cfg.Driver {
	case appconfig.ObjectStorageDriverMemory:
		return NewMemoryStore(), nil
	case appconfig.ObjectStorageDriverRustFS:
		return NewS3Store(Config{
			Driver:           cfg.Driver,
			Endpoint:         cfg.Endpoint,
			ExternalEndpoint: cfg.ExternalEndpoint,
			Bucket:           cfg.Bucket,
			Region:           cfg.Region,
			UseSSL:           cfg.UseSSL,
			ExternalUseSSL:   cfg.ExternalUseSSL,
			AccessKeyID:      cfg.AccessKeyID,
			SecretAccessKey:  cfg.SecretAccessKey,
		})
	default:
		return nil, fmt.Errorf("unsupported object storage driver %q", cfg.Driver)
	}
}
