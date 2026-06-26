package config

// AdminBootstrapConfig 是 admin-api 启动时幂等确保后台管理员账号与首次登录凭据的配置
// （#663：从 pkg/config 搬到 admin 域属主，仅本服务消费）。
type AdminBootstrapConfig struct {
	Identifier  string
	Password    string
	DisplayName string
}

func DefaultAdminBootstrapConfig() AdminBootstrapConfig {
	return AdminBootstrapConfig{
		Identifier:  "amin",
		DisplayName: "管理后台管理员",
	}
}
