package svc

import (
	"context"
	"database/sql"
	"testing"

	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	sharemodel "github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// fakeAccountStore 是 accounts⋈profiles 的进程内共享存储，供 fake accounts/profiles model 复用，
// 使适配器在「事务」内对两个 model 的写落到同一份数据上（fake 不需要真事务）。
type fakeAccountStore struct {
	byID map[string]*model.AccountProfile
}

// fakeAccountsModel 仅实现适配器用到的方法；其余经内嵌接口在未用时 nil（被调则 panic 暴露遗漏）。
type fakeAccountsModel struct {
	model.AccountsModel
	store *fakeAccountStore
}

func (m *fakeAccountsModel) WithSession(sqlx.Session) model.AccountsModel { return m }

func (m *fakeAccountsModel) Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	return fn(ctx, nil)
}

func (m *fakeAccountsModel) Insert(_ context.Context, data *model.Accounts) (sql.Result, error) {
	rec := m.store.byID[data.AccountId]
	if rec == nil {
		rec = &model.AccountProfile{AccountID: data.AccountId}
		m.store.byID[data.AccountId] = rec
	}
	rec.Identifier = data.Identifier
	rec.AccountType = data.AccountType
	rec.EmailNormalized = data.EmailNormalized
	rec.EmailVerifiedAt = data.EmailVerifiedAt
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

func (m *fakeAccountsModel) ExistsByIdentifier(_ context.Context, identifier string) (bool, error) {
	for _, rec := range m.store.byID {
		if rec.Identifier == identifier {
			return true, nil
		}
	}
	return false, nil
}

func (m *fakeAccountsModel) ListAccountProfilesByType(_ context.Context, accountType int64) ([]*model.AccountProfile, error) {
	out := make([]*model.AccountProfile, 0, len(m.store.byID))
	for _, rec := range m.store.byID {
		if rec.AccountType == accountType {
			clone := *rec
			out = append(out, &clone)
		}
	}
	return out, nil
}

func (m *fakeAccountsModel) RenameIdentifier(_ context.Context, fromIdentifier, toIdentifier string) (*model.AccountProfile, error) {
	for _, rec := range m.store.byID {
		if rec.Identifier == fromIdentifier {
			rec.Identifier = toIdentifier
			clone := *rec
			return &clone, nil
		}
	}
	return nil, model.ErrNotFound
}

// fakeProfilesModel 仅实现适配器用到的方法。
type fakeProfilesModel struct {
	model.ProfilesModel
	store *fakeAccountStore
}

func (m *fakeProfilesModel) WithSession(sqlx.Session) model.ProfilesModel { return m }

func (m *fakeProfilesModel) InsertProfile(_ context.Context, in model.ProfileInsert) error {
	rec := m.store.byID[in.AccountID]
	if rec == nil {
		rec = &model.AccountProfile{AccountID: in.AccountID}
		m.store.byID[in.AccountID] = rec
	}
	rec.DisplayName = in.DisplayName
	rec.Name = in.Name
	rec.Gender = in.Gender
	rec.Region = in.Region
	rec.AvatarMediaID = in.AvatarMediaID
	rec.AvatarURL = in.AvatarURL
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
	return nil
}

func newFakeRepo() (*assistantAccountRepo, *fakeAccountStore, *fakeAccountsModel) {
	store := &fakeAccountStore{byID: map[string]*model.AccountProfile{}}
	accounts := &fakeAccountsModel{store: store}
	profiles := &fakeProfilesModel{store: store}
	repo := newAssistantAccountRepo(accounts, profiles, repository.FriendshipRepository(nil))
	return repo, store, accounts
}

func TestAssistantAccountRepoCreateAndGet(t *testing.T) {
	repo, _, _ := newFakeRepo()
	ctx := context.Background()

	created, err := repo.Create(ctx, sharemodel.User{
		Identifier:  "agent_creator",
		DisplayName: "AI 助手",
		Name:        "agent_creator",
		Gender:      "unknown",
		AccountType: sharemodel.AccountTypeAgent,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.AccountID == "" {
		t.Fatal("Create: expected generated account id")
	}
	if created.AccountType != sharemodel.AccountTypeAgent {
		t.Fatalf("Create: account_type = %q, want agent", created.AccountType)
	}

	got, err := repo.GetByID(ctx, created.AccountID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Identifier != "agent_creator" || got.DisplayName != "AI 助手" {
		t.Fatalf("GetByID mismatch: %+v", got)
	}

	byIdent, err := repo.GetByIdentifier(ctx, "agent_creator")
	if err != nil {
		t.Fatalf("GetByIdentifier: %v", err)
	}
	if byIdent.AccountID != created.AccountID {
		t.Fatalf("GetByIdentifier id = %q, want %q", byIdent.AccountID, created.AccountID)
	}
}

func TestAssistantAccountRepoGetByIDNotFound(t *testing.T) {
	repo, _, _ := newFakeRepo()
	_, err := repo.GetByID(context.Background(), "missing")
	if apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("GetByID(missing) error = %v, want NotFound", err)
	}
}

func TestAssistantAccountRepoListByAccountType(t *testing.T) {
	repo, store, _ := newFakeRepo()
	store.byID["u1"] = &model.AccountProfile{AccountID: "u1", Identifier: "u1", AccountType: model.AccountTypeUser}
	store.byID["a1"] = &model.AccountProfile{AccountID: "a1", Identifier: "a1", AccountType: model.AccountTypeAgent}

	users, err := repo.ListByAccountType(context.Background(), sharemodel.AccountTypeUser)
	if err != nil {
		t.Fatalf("ListByAccountType: %v", err)
	}
	if len(users) != 1 || users[0].AccountID != "u1" {
		t.Fatalf("ListByAccountType = %+v, want only u1", users)
	}
}

func TestAssistantAccountRepoRenameAndUpdate(t *testing.T) {
	repo, store, _ := newFakeRepo()
	store.byID["a1"] = &model.AccountProfile{AccountID: "a1", Identifier: "agent_father", AccountType: model.AccountTypeAgent}
	ctx := context.Background()

	renamed, err := repo.RenameIdentifier(ctx, "agent_father", "agent_creator")
	if err != nil {
		t.Fatalf("RenameIdentifier: %v", err)
	}
	if renamed.Identifier != "agent_creator" {
		t.Fatalf("RenameIdentifier identifier = %q, want agent_creator", renamed.Identifier)
	}

	dn := "AI 助手"
	updated, err := repo.UpdateProfile(ctx, "a1", repository.AccountProfilePatch{DisplayName: &dn})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if updated.DisplayName != "AI 助手" {
		t.Fatalf("UpdateProfile display_name = %q", updated.DisplayName)
	}
}
