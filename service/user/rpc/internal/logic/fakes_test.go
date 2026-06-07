package logic

import (
	"context"
	"database/sql"
	"time"

	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var fixedTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// fakeStore 是 accounts⋈profiles 的进程内共享存储，供 fake accounts/profiles model 复用，
// 这样 Logic 在「事务」内对两个 model 的写入落在同一份数据上（fake 不需要真事务）。
type fakeStore struct {
	byID map[string]*model.AccountProfile
}

func newFakeStore() *fakeStore {
	return &fakeStore{byID: map[string]*model.AccountProfile{}}
}

// --- fakeAccountsModel implements model.AccountsModel ---

type fakeAccountsModel struct {
	store *fakeStore
}

func newFakeAccountsModel(store *fakeStore) *fakeAccountsModel {
	return &fakeAccountsModel{store: store}
}

func (m *fakeAccountsModel) WithSession(_ sqlx.Session) model.AccountsModel { return m }

func (m *fakeAccountsModel) Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	return fn(ctx, nil)
}

func (m *fakeAccountsModel) Insert(_ context.Context, data *model.Accounts) (sql.Result, error) {
	rec := m.store.byID[data.AccountId]
	if rec == nil {
		rec = &model.AccountProfile{}
		m.store.byID[data.AccountId] = rec
	}
	rec.AccountID = data.AccountId
	rec.Identifier = data.Identifier
	rec.AccountType = data.AccountType
	rec.EmailNormalized = data.EmailNormalized
	rec.EmailVerifiedAt = data.EmailVerifiedAt
	rec.AccountCreatedAt = fixedTime
	rec.AccountUpdatedAt = fixedTime
	return nil, nil
}

func (m *fakeAccountsModel) FindAccountProfileByID(_ context.Context, accountID string) (*model.AccountProfile, error) {
	if rec, ok := m.store.byID[accountID]; ok {
		clone := *rec
		return &clone, nil
	}
	return nil, model.ErrNotFound
}

func (m *fakeAccountsModel) FindAccountProfileByIdentifier(_ context.Context, identifier string) (*model.AccountProfile, error) {
	for _, rec := range m.store.byID {
		if rec.Identifier == identifier {
			clone := *rec
			return &clone, nil
		}
	}
	return nil, model.ErrNotFound
}

func (m *fakeAccountsModel) ListAccountProfilesByIDs(_ context.Context, accountIDs []string) ([]*model.AccountProfile, error) {
	seen := map[string]struct{}{}
	out := make([]*model.AccountProfile, 0, len(accountIDs))
	for _, id := range accountIDs {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if rec, ok := m.store.byID[id]; ok {
			clone := *rec
			out = append(out, &clone)
		}
	}
	return out, nil
}

func (m *fakeAccountsModel) ExistsByIdentifier(_ context.Context, identifier string) (bool, error) {
	for _, rec := range m.store.byID {
		if rec.Identifier == identifier {
			return true, nil
		}
	}
	return false, nil
}

// gen accountsModel methods unused by Logic unit tests.
func (m *fakeAccountsModel) FindOne(context.Context, string) (*model.Accounts, error) {
	panic("fakeAccountsModel.FindOne: unused")
}
func (m *fakeAccountsModel) FindOneByEmailNormalized(context.Context, string) (*model.Accounts, error) {
	panic("fakeAccountsModel.FindOneByEmailNormalized: unused")
}
func (m *fakeAccountsModel) FindOneByIdentifier(context.Context, string) (*model.Accounts, error) {
	panic("fakeAccountsModel.FindOneByIdentifier: unused")
}
func (m *fakeAccountsModel) Update(context.Context, *model.Accounts) error {
	panic("fakeAccountsModel.Update: unused")
}
func (m *fakeAccountsModel) Delete(context.Context, string) error {
	panic("fakeAccountsModel.Delete: unused")
}

// --- fakeProfilesModel implements model.ProfilesModel ---

type fakeProfilesModel struct {
	store *fakeStore
}

func newFakeProfilesModel(store *fakeStore) *fakeProfilesModel {
	return &fakeProfilesModel{store: store}
}

func (m *fakeProfilesModel) WithSession(_ sqlx.Session) model.ProfilesModel { return m }

func (m *fakeProfilesModel) InsertProfile(_ context.Context, in model.ProfileInsert) error {
	rec := m.store.byID[in.AccountID]
	if rec == nil {
		rec = &model.AccountProfile{AccountID: in.AccountID}
		m.store.byID[in.AccountID] = rec
	}
	rec.DisplayName = in.DisplayName
	rec.Name = in.Name
	rec.Gender = in.Gender
	rec.BirthDate = in.BirthDate
	rec.Region = in.Region
	rec.AvatarMediaID = in.AvatarMediaID
	rec.AvatarURL = in.AvatarURL
	rec.ProfileCreatedAt = fixedTime
	rec.ProfileUpdatedAt = fixedTime
	return nil
}

func (m *fakeProfilesModel) UpdateProfileFields(_ context.Context, accountID string, patch model.ProfilePatch) error {
	rec, ok := m.store.byID[accountID]
	if !ok {
		return model.ErrNotFound
	}
	if patch.DisplayName != nil {
		rec.DisplayName = *patch.DisplayName
	}
	if patch.Name != nil {
		rec.Name = *patch.Name
	}
	if patch.Gender != nil {
		rec.Gender = *patch.Gender
	}
	if patch.BirthDate != nil {
		rec.BirthDate = *patch.BirthDate
	}
	if patch.Region != nil {
		rec.Region = *patch.Region
	}
	return nil
}

func (m *fakeProfilesModel) UpdateAvatar(_ context.Context, accountID, avatarMediaID, avatarURL string) error {
	rec, ok := m.store.byID[accountID]
	if !ok {
		return model.ErrNotFound
	}
	rec.AvatarMediaID = avatarMediaID
	rec.AvatarURL = avatarURL
	return nil
}

// gen profilesModel methods unused by Logic unit tests.
func (m *fakeProfilesModel) Insert(context.Context, *model.Profiles) (sql.Result, error) {
	panic("fakeProfilesModel.Insert: unused")
}
func (m *fakeProfilesModel) FindOne(context.Context, string) (*model.Profiles, error) {
	panic("fakeProfilesModel.FindOne: unused")
}
func (m *fakeProfilesModel) Update(context.Context, *model.Profiles) error {
	panic("fakeProfilesModel.Update: unused")
}
func (m *fakeProfilesModel) Delete(context.Context, string) error {
	panic("fakeProfilesModel.Delete: unused")
}

// --- keystone fakes ---

type fakeProvisioner struct {
	ensuredFor []string
}

func (f *fakeProvisioner) EnsureForUser(_ context.Context, accountID string) error {
	f.ensuredFor = append(f.ensuredFor, accountID)
	return nil
}

type fakeAvatarValidator struct {
	calls int
	err   error
}

func (f *fakeAvatarValidator) ValidateAvatarMedia(_ context.Context, _ string, _ string) error {
	f.calls++
	return f.err
}
