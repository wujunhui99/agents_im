// Package svc 汇总 resource-rpc 两个能力域的 ServiceContext：media 与 mail。
// 两者各自保持独立的依赖（DB/对象存储 vs 外部供应商），但共享同一个进程与端口。
package svc

import (
	"github.com/wujunhui99/agents_im/service/resource/rpc/internal/config"
	mailconfig "github.com/wujunhui99/agents_im/service/resource/rpc/internal/mail/config"
	mailsvc "github.com/wujunhui99/agents_im/service/resource/rpc/internal/mail/svc"
	mediaconfig "github.com/wujunhui99/agents_im/service/resource/rpc/internal/media/config"
	mediasvc "github.com/wujunhui99/agents_im/service/resource/rpc/internal/media/svc"
)

type ServiceContext struct {
	Config config.Config
	Media  *mediasvc.ServiceContext
	Mail   *mailsvc.ServiceContext
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
		Media: mediasvc.NewServiceContext(mediaconfig.Config{
			DataSource:    c.DataSource,
			ObjectStorage: c.ObjectStorage,
			Snowflake:     c.Snowflake,
		}),
		Mail: mailsvc.NewServiceContext(mailconfig.Config{
			TencentSES: c.TencentSES,
		}),
	}
}
