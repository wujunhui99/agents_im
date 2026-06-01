package svc

import (
	"log"

	business "github.com/wujunhui99/agents_im/service/media/core"
	"github.com/wujunhui99/agents_im/internal/repository"
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/objectstorage"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/config"
)

// ServiceContext owns the media/object-storage lifecycle. MediaLogic is wired
// with a live object store + media repository; account/message repositories back
// download access control. Media object validation reads the shared media_objects
// table; media-rpc owns the writes and the object store.
type ServiceContext struct {
	Config     config.Config
	MediaLogic *business.MediaLogic
}

func NewServiceContext(c config.Config) *ServiceContext {
	mediaRepo, err := repository.NewMediaRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build media repository: %v", err)
	}
	accountRepo, err := repository.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build account repository: %v", err)
	}
	messageRepo, err := repository.NewMessageRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build message repository: %v", err)
	}

	osCfg, err := appconfig.ResolveObjectStorageConfig(c.ObjectStorage, c.StorageDriver)
	if err != nil {
		log.Fatalf("resolve object storage config: %v", err)
	}
	objectStore, err := objectstorage.NewStore(osCfg)
	if err != nil {
		log.Fatalf("build object storage: %v", err)
	}

	mediaLogic := business.NewMediaLogic(mediaRepo, objectStore, osCfg.Bucket).
		WithAccountRepository(accountRepo).
		WithAttachmentAccessChecker(business.NewMessageMediaAccessChecker(messageRepo))

	return &ServiceContext{
		Config:     c,
		MediaLogic: mediaLogic,
	}
}
