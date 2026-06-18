package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/svc"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type CreateUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateUserLogic {
	return &CreateUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateUserLogic) CreateUser(in *userpb.CreateUserRequest) (*userpb.UserResponse, error) {
	identifier, err := validateIdentifier(in.GetIdentifier())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	displayName, name, err := resolveNames(in.GetDisplayName(), in.GetName(), identifier)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	gender, err := validateGender(in.GetGender())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	region, err := validateRegion(in.GetRegion())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	accountTypeDB, err := accountTypeToDB(in.GetAccountType())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	accountID, err := idgen.NewAccountString(facetForAccountType(accountTypeDB))
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	// email_verified_at：auth 注册校验邮箱后带入（RFC3339，空=未验证）。落 accounts。
	emailVerifiedAt, err := parseEmailVerifiedAt(in.GetEmailVerifiedAt())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	// 事务边界在 Logic 层：accounts + profiles 两行要么全成、要么全败。
	var created *model.AccountProfile
	err = l.svcCtx.Accounts.Transact(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		accounts := l.svcCtx.Accounts.WithSession(session)
		profiles := l.svcCtx.Profiles.WithSession(session)

		if _, err := accounts.Insert(ctx, &model.Accounts{
			AccountId:       accountID,
			Identifier:      identifier,
			AccountType:     accountTypeDB,
			EmailNormalized: strings.TrimSpace(in.GetEmail()),
			EmailVerifiedAt: emailVerifiedAt,
		}); err != nil {
			return err
		}
		if err := profiles.InsertProfile(ctx, model.ProfileInsert{
			AccountID:   accountID,
			DisplayName: displayName,
			Name:        name,
			Gender:      genderToDB(gender),
			BirthDate:   strings.TrimSpace(in.GetBirthDate()),
			Region:      region,
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

	// keystone：新建 user 账号后开通默认助手（agent 域写）。仅 account_type=user 触发，
	// 与 monolith 行为一致；实现内部仍会二次校验。无 agent-rpc 可 BFF，待迁移后删。
	if created.AccountType == model.AccountTypeUser {
		if err := l.svcCtx.Assistant.EnsureForUser(l.ctx, created.AccountID); err != nil {
			return nil, rpcerror.ToStatus(err)
		}
	}

	return toUserResponse(created), nil
}
