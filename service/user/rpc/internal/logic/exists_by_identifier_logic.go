package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/svc"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

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
