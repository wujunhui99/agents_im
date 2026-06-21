package svc

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	sharemodel "github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// assistantAccountRepo 是 gate #550 的第 3 处（最后一处）存活 avatar 读路径切割：
// 默认助手 provisioner（keystone agent 域写，寄居 internal/logic）期望注入一个
// repository.Repository，旧实现整块走 internal/repository——其 accounts⋈profiles 读写都
// 把 profiles.avatar_media_id scan 进 Go string，列改 bigint 后 pgx v5 严格类型会 runtime 失败。
//
// 本适配器把其中「账号读写」改由 user-rpc 自有 goctl model（AccountsModel/ProfilesModel）承接——
// 这些 model 由 #550 随列变更一并 goctl 重生（string→int64），故 bigint 化后仍正确。好友方法
// （friendships 表无 avatar，非 #550 blocker）经内嵌 FriendshipRepository 委托回 internal postgres
// repo；agent provisioner 的 agent/registry 写仍由 internal/repository 承接（agent keystone，
// 无 avatar），待 agent 域迁移后彻底删（继 #551 auth、#553 agent-api、#555 msg-rpc、#589 media）。
type assistantAccountRepo struct {
	repository.FriendshipRepository
	accounts model.AccountsModel
	profiles model.ProfilesModel
}

var _ repository.Repository = (*assistantAccountRepo)(nil)

// newAssistantAccountRepo 组合 user-rpc 自有账号 model 与内部好友 repo 委托。
func newAssistantAccountRepo(accounts model.AccountsModel, profiles model.ProfilesModel, friendships repository.FriendshipRepository) *assistantAccountRepo {
	return &assistantAccountRepo{FriendshipRepository: friendships, accounts: accounts, profiles: profiles}
}

func (r *assistantAccountRepo) Create(ctx context.Context, account sharemodel.User) (sharemodel.User, error) {
	accountType, ok := sharemodel.NormalizeAccountType(string(account.AccountType))
	if !ok {
		return sharemodel.User{}, apperror.InvalidArgument("account_type must be user, agent, admin, or test")
	}
	accountTypeDB := accountTypeToDBInt(accountType)

	accountID := strings.TrimSpace(account.AccountID)
	if accountID == "" {
		generated, err := idgen.NewAccountString(facetForAccountTypeInt(accountTypeDB))
		if err != nil {
			return sharemodel.User{}, err
		}
		accountID = generated
	}

	// wire 头像 media id 是十进制串、DB 是 bigint(#550):转成 int64 落库(空→0 无头像)。
	avatarMediaID, err := model.ParseAvatarMediaID(account.AvatarMediaID)
	if err != nil {
		return sharemodel.User{}, apperror.InvalidArgument("avatar_media_id must be a decimal media id")
	}

	// 事务边界在此（适配器扮演 Logic 角色）：accounts + profiles 两行原子写。
	var created *model.AccountProfile
	err = r.accounts.Transact(ctx, func(ctx context.Context, session sqlx.Session) error {
		accounts := r.accounts.WithSession(session)
		profiles := r.profiles.WithSession(session)

		if _, err := accounts.Insert(ctx, &model.Accounts{
			AccountId:       accountID,
			Identifier:      account.Identifier,
			AccountType:     accountTypeDB,
			EmailNormalized: strings.TrimSpace(account.Email),
			EmailVerifiedAt: nullableTime(account.EmailVerifiedAt),
		}); err != nil {
			return err
		}
		if err := profiles.InsertProfile(ctx, model.ProfileInsert{
			AccountID:     accountID,
			DisplayName:   account.DisplayName,
			Name:          account.Name,
			Gender:        genderToDBInt(account.Gender),
			BirthDate:     account.BirthDate,
			Region:        account.Region,
			AvatarMediaID: avatarMediaID,
			AvatarURL:     strings.TrimSpace(account.AvatarURL),
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
		return sharemodel.User{}, mapAssistantWriteError(err)
	}
	return toShareUser(created), nil
}

func (r *assistantAccountRepo) GetByID(ctx context.Context, accountID string) (sharemodel.User, error) {
	ap, err := r.accounts.FindAccountProfileByID(ctx, accountID)
	if err != nil {
		return sharemodel.User{}, mapAssistantReadError(err)
	}
	return toShareUser(ap), nil
}

func (r *assistantAccountRepo) GetByIdentifier(ctx context.Context, identifier string) (sharemodel.User, error) {
	ap, err := r.accounts.FindAccountProfileByIdentifier(ctx, identifier)
	if err != nil {
		return sharemodel.User{}, mapAssistantReadError(err)
	}
	return toShareUser(ap), nil
}

func (r *assistantAccountRepo) ExistsByIdentifier(ctx context.Context, identifier string) (bool, error) {
	return r.accounts.ExistsByIdentifier(ctx, identifier)
}

func (r *assistantAccountRepo) ListByIDs(ctx context.Context, accountIDs []string) ([]sharemodel.User, error) {
	aps, err := r.accounts.ListAccountProfilesByIDs(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	return toShareUsers(aps), nil
}

func (r *assistantAccountRepo) ListByAccountType(ctx context.Context, accountType sharemodel.AccountType) ([]sharemodel.User, error) {
	normalized, ok := sharemodel.NormalizeAccountType(string(accountType))
	if !ok {
		return nil, apperror.InvalidArgument("account_type must be user, agent, admin, or test")
	}
	aps, err := r.accounts.ListAccountProfilesByType(ctx, accountTypeToDBInt(normalized))
	if err != nil {
		return nil, err
	}
	return toShareUsers(aps), nil
}

func (r *assistantAccountRepo) RenameIdentifier(ctx context.Context, fromIdentifier, toIdentifier string) (sharemodel.User, error) {
	from := strings.ToLower(strings.TrimSpace(fromIdentifier))
	to := strings.ToLower(strings.TrimSpace(toIdentifier))
	ap, err := r.accounts.RenameIdentifier(ctx, from, to)
	if err != nil {
		return sharemodel.User{}, mapAssistantWriteError(err)
	}
	return toShareUser(ap), nil
}

func (r *assistantAccountRepo) UpdateProfile(ctx context.Context, accountID string, patch repository.AccountProfilePatch) (sharemodel.User, error) {
	if err := r.profiles.UpdateProfileFields(ctx, accountID, toModelProfilePatch(patch)); err != nil {
		return sharemodel.User{}, mapAssistantWriteError(err)
	}
	return r.GetByID(ctx, accountID)
}

func (r *assistantAccountRepo) UpdateAvatar(ctx context.Context, accountID, avatarMediaID, avatarURL string) (sharemodel.User, error) {
	// repository.Repository 接口的 avatarMediaID 仍是 wire 十进制串;DB 是 bigint(#550)→ 转 int64。
	mediaID, err := model.ParseAvatarMediaID(avatarMediaID)
	if err != nil {
		return sharemodel.User{}, apperror.InvalidArgument("avatar_media_id must be a decimal media id")
	}
	if err := r.profiles.UpdateAvatar(ctx, accountID, mediaID, avatarURL); err != nil {
		return sharemodel.User{}, mapAssistantWriteError(err)
	}
	return r.GetByID(ctx, accountID)
}

// --- 映射：goctl model.AccountProfile ↔ pkg/model.User，DB 整型 ↔ 字符串取值 ---

func toShareUsers(aps []*model.AccountProfile) []sharemodel.User {
	users := make([]sharemodel.User, 0, len(aps))
	for _, ap := range aps {
		users = append(users, toShareUser(ap))
	}
	return users
}

func toShareUser(ap *model.AccountProfile) sharemodel.User {
	if ap == nil {
		return sharemodel.User{}
	}
	var emailVerifiedAt time.Time
	if ap.EmailVerifiedAt.Valid {
		emailVerifiedAt = ap.EmailVerifiedAt.Time.UTC()
	}
	return sharemodel.NewAccountProfile(
		sharemodel.Account{
			AccountID:       ap.AccountID,
			Identifier:      ap.Identifier,
			Email:           ap.EmailNormalized,
			EmailVerifiedAt: emailVerifiedAt,
			AccountType:     accountTypeFromDBInt(ap.AccountType),
			CreatedAt:       ap.AccountCreatedAt.UTC(),
			UpdatedAt:       ap.AccountUpdatedAt.UTC(),
		},
		sharemodel.Profile{
			AccountID:     ap.AccountID,
			DisplayName:   ap.DisplayName,
			Name:          ap.Name,
			Gender:        genderFromDBInt(ap.Gender),
			BirthDate:     ap.BirthDate,
			Region:        ap.Region,
			AvatarMediaID: model.FormatAvatarMediaID(ap.AvatarMediaID),
			AvatarURL:     ap.AvatarURL,
			CreatedAt:     ap.ProfileCreatedAt.UTC(),
			UpdatedAt:     ap.ProfileUpdatedAt.UTC(),
		},
	)
}

func toModelProfilePatch(patch repository.AccountProfilePatch) model.ProfilePatch {
	out := model.ProfilePatch{
		DisplayName: patch.DisplayName,
		Name:        patch.Name,
		BirthDate:   patch.BirthDate,
		Region:      patch.Region,
	}
	if patch.Gender != nil {
		g := genderToDBInt(*patch.Gender)
		out.Gender = &g
	}
	return out
}

func accountTypeToDBInt(t sharemodel.AccountType) int64 {
	switch t {
	case sharemodel.AccountTypeAdmin:
		return model.AccountTypeAdmin
	case sharemodel.AccountTypeAgent:
		return model.AccountTypeAgent
	case sharemodel.AccountTypeTest:
		return model.AccountTypeTest
	default:
		return model.AccountTypeUser
	}
}

func accountTypeFromDBInt(v int64) sharemodel.AccountType {
	switch v {
	case model.AccountTypeAdmin:
		return sharemodel.AccountTypeAdmin
	case model.AccountTypeAgent:
		return sharemodel.AccountTypeAgent
	case model.AccountTypeTest:
		return sharemodel.AccountTypeTest
	default:
		return sharemodel.AccountTypeUser
	}
}

func facetForAccountTypeInt(accountTypeDB int64) idgen.Facet {
	if accountTypeDB == model.AccountTypeAgent {
		return idgen.FacetAgent
	}
	return idgen.FacetHuman
}

func genderToDBInt(g string) int64 {
	switch g {
	case "male":
		return model.GenderMale
	case "female":
		return model.GenderFemale
	case "other":
		return model.GenderOther
	default:
		return model.GenderUnknown
	}
}

func genderFromDBInt(v int64) string {
	switch v {
	case model.GenderMale:
		return "male"
	case model.GenderFemale:
		return "female"
	case model.GenderOther:
		return "other"
	default:
		return "unknown"
	}
}

func nullableTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}
}

func mapAssistantReadError(err error) error {
	if errors.Is(err, model.ErrNotFound) {
		return apperror.NotFound("account not found")
	}
	return err
}

func mapAssistantWriteError(err error) error {
	switch {
	case errors.Is(err, model.ErrNotFound):
		return apperror.NotFound("account not found")
	case model.IsUniqueViolation(err):
		return apperror.AlreadyExists("identifier already exists")
	case model.IsCheckViolation(err):
		return apperror.InvalidArgument("invalid account profile or account_type")
	default:
		return err
	}
}
