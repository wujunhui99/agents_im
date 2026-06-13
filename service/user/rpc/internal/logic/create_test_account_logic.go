package logic

import (
	"context"
	"errors"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/svc"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type CreateTestAccountLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTestAccountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTestAccountLogic {
	return &CreateTestAccountLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateTestAccount 创建管理后台用的测试账户（account_type=test，不绑定邮箱）。
// 只建 accounts+profiles；登录凭据是 auth 域数据，由 admin-api BFF 编排调用
// auth-rpc EnsureTestCredential 写入。
// 幂等：identifier 已存在且为 test 账户时返回该账户（already_exists=true），便于
// BFF 在「建号成功但设凭据失败」后重试，也支撑同名重复创建 = 重置密码的语义。
func (l *CreateTestAccountLogic) CreateTestAccount(in *userpb.CreateTestAccountRequest) (*userpb.CreateTestAccountResponse, error) {
	identifier, err := validateIdentifier(in.GetIdentifier())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	displayName, name, err := resolveNames(in.GetDisplayName(), "", identifier)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	if existing, err := l.svcCtx.Accounts.FindAccountProfileByIdentifier(l.ctx, identifier); err == nil {
		if existing.AccountType != model.AccountTypeTest {
			return nil, rpcerror.ToStatus(apperror.AlreadyExists("identifier already exists as non-test account"))
		}
		return &userpb.CreateTestAccountResponse{User: toUserEntity(existing), AlreadyExists: true}, nil
	} else if !errors.Is(err, model.ErrNotFound) {
		return nil, rpcerror.ToStatus(err)
	}

	accountID, err := idgen.NewAccountString(facetForAccountType(model.AccountTypeTest))
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	// 事务边界在 Logic 层：accounts + profiles 两行要么全成、要么全败。
	var created *model.AccountProfile
	err = l.svcCtx.Accounts.Transact(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		accounts := l.svcCtx.Accounts.WithSession(session)
		profiles := l.svcCtx.Profiles.WithSession(session)

		if _, err := accounts.Insert(ctx, &model.Accounts{
			AccountId:   accountID,
			Identifier:  identifier,
			AccountType: model.AccountTypeTest,
		}); err != nil {
			return err
		}
		if err := profiles.InsertProfile(ctx, model.ProfileInsert{
			AccountID:   accountID,
			DisplayName: displayName,
			Name:        name,
			Gender:      model.GenderUnknown,
		}); err != nil {
			return err
		}
		ap, err := accounts.FindAccountProfileByID(ctx, accountID)
		if err != nil {
			return err
		}
		created = ap
		return nil
	})
	if err != nil {
		return nil, rpcerror.ToStatus(mapAccountWriteError(err))
	}

	// keystone：测试账户与 user 一致开通默认助手（agent 域写），便于测试 AI 链路。
	if err := l.svcCtx.Assistant.EnsureForUser(l.ctx, created.AccountID); err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	return &userpb.CreateTestAccountResponse{User: toUserEntity(created)}, nil
}
