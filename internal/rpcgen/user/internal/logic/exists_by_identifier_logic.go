package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/rpcgen/user/internal/svc"
	"github.com/wujunhui99/agents_im/proto/userpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExistsByIdentifierLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExistsByIdentifierLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExistsByIdentifierLogic {
	return &ExistsByIdentifierLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ExistsByIdentifierLogic) ExistsByIdentifier(in *userpb.ExistsByIdentifierRequest) (*userpb.ExistsByIdentifierResponse, error) {
	// todo: add your logic here and delete this line

	return &userpb.ExistsByIdentifierResponse{}, nil
}
