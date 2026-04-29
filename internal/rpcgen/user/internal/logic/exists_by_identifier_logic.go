package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
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
	result, err := l.svcCtx.UserLogic.ExistsByIdentifier(l.ctx, business.ExistsByIdentifierRequest{
		Identifier: in.GetIdentifier(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &userpb.ExistsByIdentifierResponse{
		Identifier: result.Identifier,
		Exists:     result.Exists,
	}, nil
}
