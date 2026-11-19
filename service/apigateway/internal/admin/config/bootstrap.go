package config

// AdminBootstrapConfig 是 admin-api 启动时幂等确保后台管理员账号与首次登录凭据的配置
// （#663：从 pkg/config 搬到 admin 域属主，仅本服务消费）。#664：默认值/env 覆盖改用
// go-zero struct tag（default=/env=，env 用裸名 ADMIN_BOOTSTRAP_*），删手写 Default* 函数。
type AdminBootstrapConfig struct {
	Identifier  string `json:",default=amin,env=ADMIN_BOOTSTRAP_IDENTIFIER"`
	Password    string `json:",optional,env=ADMIN_BOOTSTRAP_PASSWORD"`
	DisplayName string `json:",default=管理后台管理员,env=ADMIN_BOOTSTRAP_DISPLAY_NAME"`
}
