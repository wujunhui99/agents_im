package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ McpServersModel = (*customMcpServersModel)(nil)

type (
	// McpServersModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMcpServersModel.
	McpServersModel interface {
		mcpServersModel
		withSession(session sqlx.Session) McpServersModel
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

func (m *customMcpServersModel) withSession(session sqlx.Session) McpServersModel {
	return NewMcpServersModel(sqlx.NewSqlConnFromSession(session))
}
