package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/svc"
	userpb "github.com/wujunhui99/agents_im_protocol/user/v1"

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

// publishUserCreated 是尽力而为的事件发布：账号已落库，发布失败只记录日志，不回滚
// 注册（重试注册会命中 already_exists，反而丢掉事件）。消费端按 account_id 幂等。
func (l *CreateUserLogic) publishUserCreated(account *model.AccountProfile) {
	if account.AccountType != model.AccountTypeUser && account.AccountType != model.AccountTypeTest {
		return
	}
	if l.svcCtx.Events == nil {
		logx.WithContext(l.ctx).Errorf("user.created not published: event publisher is not configured account=%s", account.AccountID)
		return
	}
	if err := l.svcCtx.Events.PublishUserCreated(l.ctx, account.AccountID, accountTypeFromDB(account.AccountType)); err != nil {
		logx.WithContext(l.ctx).Errorf("publish user.created failed account=%s: %v", account.AccountID, err)
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

	// 新建账号后发布 user.created.v1：默认助手绑定（agent 域）+ 好友关系由 agent-rpc
	// 消费事件完成。user-rpc 不再同步调用 agent-rpc/friends-rpc（断调用环）。
	l.publishUserCreated(created)

	return toUserResponse(created), nil
}
