package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ McpServersModel = (*customMcpServersModel)(nil)

type (
	// McpServersModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMcpServersModel.
	McpServersModel interface {
		mcpServersModel

		// InsertReturning 插入并返回库生成的完整行（server_id 自增、jsonb config 列）。
		InsertReturning(ctx context.Context, data *McpServers) (*McpServers, error)
	}

	customMcpServersModel struct {
		*defaultMcpServersModel
	}
)

// NewMcpServersModel returns a model for the database table.
func NewMcpServersModel(conn sqlx.SqlConn) McpServersModel {
	return &customMcpServersModel{
		defaultMcpServersModel: newMcpServersModel(conn),
	}
}
