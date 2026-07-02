package svc

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	sharemodel "github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// AccountProfilePatch 是账号资料的可选字段补丁（user 域本地，脱 internal/repository，#618 顶层
// internal/ 退役）。原 repository.AccountProfilePatch 随 internal/ 删除，本地承接。
type AccountProfilePatch struct {
	DisplayName *string
	Name        *string
	Gender      *string
	BirthDate   *string
	Region      *string
}

// assistantAccountRepo 是默认助手账号读写适配器（gate #550 avatar bigint 化的最后切割处）：
// 账号读写由 user-rpc 自有 goctl model（AccountsModel/ProfilesModel）承接——这些 model 随 #550
// 列变更一并 goctl 重生（avatar_media_id string→int64），故 bigint 化后仍正确。好友/agent 装配
// 分别经 friends-rpc / agent-rpc（#606），已无 internal/repository 依赖（#618 顶层 internal/ 退役）。
type assistantAccountRepo struct {
	accounts model.AccountsModel
	profiles model.ProfilesModel
}

// newAssistantAccountRepo 组合 user-rpc 自有账号 model。
func newAssistantAccountRepo(accounts model.AccountsModel, profiles model.ProfilesModel) *assistantAccountRepo {
	return &assistantAccountRepo{accounts: accounts, profiles: profiles}
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

func (r *assistantAccountRepo) UpdateProfile(ctx context.Context, accountID string, patch AccountProfilePatch) (sharemodel.User, error) {
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

func toModelProfilePatch(patch AccountProfilePatch) model.ProfilePatch {
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
