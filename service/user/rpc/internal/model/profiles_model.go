package model

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ProfilesModel = (*customProfilesModel)(nil)

// ProfileInsert 是写 profiles 一行的 DB-ready 输入（gender 已由 Logic 映射成整型，
// birth_date 为原始字符串，空串落 NULL）。
type ProfileInsert struct {
	AccountID     string
	DisplayName   string
	Name          string
	Gender        int64
	BirthDate     string
	Region        string
	AvatarMediaID int64
	AvatarURL     string
}

// ProfilePatch 是 profiles 局部更新的 DB-ready 补丁；nil 字段不更新。
// gender 已由 Logic 映射成整型；birth_date 空串落 NULL。
type ProfilePatch struct {
	DisplayName *string
	Name        *string
	Gender      *int64
	BirthDate   *string
	Region      *string
}

type (
	// ProfilesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customProfilesModel.
	ProfilesModel interface {
		profilesModel
		// WithSession 返回绑定到给定事务 session 的 model，供 Logic 层在事务内复用。
		WithSession(session sqlx.Session) ProfilesModel

		// InsertProfile 插入一行 profiles（birth_date 空串落 NULL）。
		InsertProfile(ctx context.Context, in ProfileInsert) error
		// UpdateProfileFields 按 patch 局部更新 profiles 并刷新 updated_at；
		// patch 无任何字段则不更新；账号不存在返回 ErrNotFound。
		UpdateProfileFields(ctx context.Context, accountID string, patch ProfilePatch) error
		// UpdateAvatar 更新头像 media id/url 并刷新 updated_at；avatarMediaID 为 DB int64
		// （0 = 无头像哨兵）；账号不存在返回 ErrNotFound。
		UpdateAvatar(ctx context.Context, accountID string, avatarMediaID int64, avatarURL string) error
	}

	customProfilesModel struct {
		*defaultProfilesModel
	}
)

// NewProfilesModel returns a model for the database table.
func NewProfilesModel(conn sqlx.SqlConn) ProfilesModel {
	return &customProfilesModel{
		defaultProfilesModel: newProfilesModel(conn),
	}
}

func (m *customProfilesModel) WithSession(session sqlx.Session) ProfilesModel {
	return NewProfilesModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customProfilesModel) InsertProfile(ctx context.Context, in ProfileInsert) error {
	_, err := m.conn.ExecCtx(ctx, `
insert into profiles (account_id, display_name, name, gender, birth_date, region, avatar_media_id, avatar_url)
values ($1, $2, $3, $4, nullif($5, '')::date, $6, $7, $8)
`, in.AccountID, in.DisplayName, in.Name, in.Gender, in.BirthDate, in.Region, in.AvatarMediaID, in.AvatarURL)
	return err
}

func (m *customProfilesModel) UpdateProfileFields(ctx context.Context, accountID string, patch ProfilePatch) error {
	setters := make([]string, 0, 5)
	args := make([]any, 0, 6)
	addSetter := func(column string, value any) {
		args = append(args, value)
		setters = append(setters, column+" = $"+strconv.Itoa(len(args)))
	}

	if patch.DisplayName != nil {
		addSetter("display_name", *patch.DisplayName)
	}
	if patch.Name != nil {
		addSetter("name", *patch.Name)
	}
	if patch.Gender != nil {
		addSetter("gender", *patch.Gender)
	}
	if patch.BirthDate != nil {
		addSetter("birth_date", sql.NullString{String: *patch.BirthDate, Valid: strings.TrimSpace(*patch.BirthDate) != ""})
	}
	if patch.Region != nil {
		addSetter("region", *patch.Region)
	}
	if len(setters) == 0 {
		return m.assertExists(ctx, accountID)
	}

	args = append(args, accountID)
	query := "update profiles set " + strings.Join(setters, ", ") +
		", updated_at = now() where account_id = $" + strconv.Itoa(len(args)) + " returning account_id"
	var returned string
	err := m.conn.QueryRowCtx(ctx, &returned, query, args...)
	switch err {
	case nil:
		return nil
	case sqlx.ErrNotFound:
		return ErrNotFound
	default:
		return err
	}
}

func (m *customProfilesModel) UpdateAvatar(ctx context.Context, accountID string, avatarMediaID int64, avatarURL string) error {
	var returned string
	err := m.conn.QueryRowCtx(ctx, &returned, `
update profiles
set avatar_media_id = $2, avatar_url = $3, updated_at = now()
where account_id = $1
returning account_id
`, accountID, avatarMediaID, avatarURL)
	switch err {
	case nil:
		return nil
	case sqlx.ErrNotFound:
		return ErrNotFound
	default:
		return err
	}
}

func (m *customProfilesModel) assertExists(ctx context.Context, accountID string) error {
	_, err := m.FindOne(ctx, accountID)
	return err
}
