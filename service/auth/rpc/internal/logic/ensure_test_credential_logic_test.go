package logic

import (
	"context"
	"database/sql"
	"testing"

	"github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// --- fakes ---

type fakeCredentialRow struct {
	hash string
	algo int64
}

type fakeCredentialsModel struct {
	rows map[string]*fakeCredentialRow
}

func newFakeCredentialsModel() *fakeCredentialsModel {
	return &fakeCredentialsModel{rows: map[string]*fakeCredentialRow{}}
}

func (m *fakeCredentialsModel) UpsertPassword(_ context.Context, accountID string, passwordHash string, passwordAlgo int64) (bool, error) {
	_, existed := m.rows[accountID]
	m.rows[accountID] = &fakeCredentialRow{hash: passwordHash, algo: passwordAlgo}
	return !existed, nil
}

func (m *fakeCredentialsModel) InsertPasswordIfAbsent(_ context.Context, accountID string, passwordHash string, passwordAlgo int64) (bool, error) {
	if _, existed := m.rows[accountID]; existed {
		return false, nil
	}
	m.rows[accountID] = &fakeCredentialRow{hash: passwordHash, algo: passwordAlgo}
	return true, nil
}

func (m *fakeCredentialsModel) WithSession(_ sqlx.Session) model.AuthCredentialsModel { return m }

func (m *fakeCredentialsModel) Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	return fn(ctx, nil)
}

func (m *fakeCredentialsModel) Insert(context.Context, *model.AuthCredentials) (sql.Result, error) {
	panic("fakeCredentialsModel.Insert: unused")
}

func (m *fakeCredentialsModel) FindOne(context.Context, string) (*model.AuthCredentials, error) {
	panic("fakeCredentialsModel.FindOne: unused")
}

func (m *fakeCredentialsModel) Update(context.Context, *model.AuthCredentials) error {
	panic("fakeCredentialsModel.Update: unused")
}

func (m *fakeCredentialsModel) Delete(context.Context, string) error {
	panic("fakeCredentialsModel.Delete: unused")
}

type fakeAccountsGuard struct {
	types map[string]int64
}

func (g *fakeAccountsGuard) FindAccountTypeByID(_ context.Context, accountID string) (int64, error) {
	if t, ok := g.types[accountID]; ok {
		return t, nil
	}
	return 0, model.ErrNotFound
}

func newEnsureTestSvc(creds *fakeCredentialsModel, guard *fakeAccountsGuard) *svc.ServiceContext {
	return &svc.ServiceContext{Credentials: creds, AccountsGuard: guard}
}

// --- tests ---

func TestEnsureTestCredentialCreatesBcryptCredential(t *testing.T) {
	creds := newFakeCredentialsModel()
	guard := &fakeAccountsGuard{types: map[string]int64{"acct-1": model.AccountTypeDBTest}}
	logic := NewEnsureTestCredentialLogic(context.Background(), newEnsureTestSvc(creds, guard))

	resp, err := logic.EnsureTestCredential(&auth.EnsureTestCredentialRequest{
		UserId:     "acct-1",
		Identifier: "tester",
		Password:   "super-secret-pw",
	})
	if err != nil {
		t.Fatalf("EnsureTestCredential: %v", err)
	}
	if resp.GetRotated() {
		t.Fatal("first ensure should not report rotated")
	}
	row := creds.rows["acct-1"]
	if row == nil {
		t.Fatal("credential row not written")
	}
	if row.algo != passwordAlgoDBBcrypt {
		t.Fatalf("password_algo = %d, want %d (bcrypt)", row.algo, passwordAlgoDBBcrypt)
	}
	// 登录校验端（internal/auth bcrypt-v1）用 bcrypt.CompareHashAndPassword 验证同一列。
	if bcrypt.CompareHashAndPassword([]byte(row.hash), []byte("super-secret-pw")) != nil {
		t.Fatal("stored hash does not verify the requested password")
	}
}

func TestEnsureTestCredentialRotatesExistingPassword(t *testing.T) {
	creds := newFakeCredentialsModel()
	guard := &fakeAccountsGuard{types: map[string]int64{"acct-1": model.AccountTypeDBTest}}
	logic := NewEnsureTestCredentialLogic(context.Background(), newEnsureTestSvc(creds, guard))

	if _, err := logic.EnsureTestCredential(&auth.EnsureTestCredentialRequest{UserId: "acct-1", Password: "first-password"}); err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	resp, err := logic.EnsureTestCredential(&auth.EnsureTestCredentialRequest{UserId: "acct-1", Password: "second-password"})
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if !resp.GetRotated() {
		t.Fatal("second ensure should report rotated")
	}
	if bcrypt.CompareHashAndPassword([]byte(creds.rows["acct-1"].hash), []byte("second-password")) != nil {
		t.Fatal("rotated hash does not verify the new password")
	}
}

func TestEnsureTestCredentialRejectsNonTestAccount(t *testing.T) {
	creds := newFakeCredentialsModel()
	guard := &fakeAccountsGuard{types: map[string]int64{"admin-1": 0, "user-1": 1}}
	logic := NewEnsureTestCredentialLogic(context.Background(), newEnsureTestSvc(creds, guard))

	for _, id := range []string{"admin-1", "user-1"} {
		if _, err := logic.EnsureTestCredential(&auth.EnsureTestCredentialRequest{UserId: id, Password: "whatever-pw"}); err == nil {
			t.Fatalf("ensure for non-test account %q should fail", id)
		}
	}
	if len(creds.rows) != 0 {
		t.Fatal("no credential should be written for non-test accounts")
	}
}

func TestEnsureTestCredentialRejectsMissingAccountAndBadInput(t *testing.T) {
	creds := newFakeCredentialsModel()
	guard := &fakeAccountsGuard{types: map[string]int64{}}
	logic := NewEnsureTestCredentialLogic(context.Background(), newEnsureTestSvc(creds, guard))

	if _, err := logic.EnsureTestCredential(&auth.EnsureTestCredentialRequest{UserId: "ghost", Password: "whatever-pw"}); err == nil {
		t.Fatal("ensure for missing account should fail")
	}
	if _, err := logic.EnsureTestCredential(&auth.EnsureTestCredentialRequest{UserId: "", Password: "whatever-pw"}); err == nil {
		t.Fatal("ensure without user_id should fail")
	}
	if _, err := logic.EnsureTestCredential(&auth.EnsureTestCredentialRequest{UserId: "ghost", Password: "short"}); err == nil {
		t.Fatal("ensure with short password should fail")
	}
}

func TestEnsureAdminCredentialCreatesBcryptCredential(t *testing.T) {
	creds := newFakeCredentialsModel()
	guard := &fakeAccountsGuard{types: map[string]int64{"admin-1": model.AccountTypeDBAdmin}}
	logic := NewEnsureAdminCredentialLogic(context.Background(), newEnsureTestSvc(creds, guard))

	resp, err := logic.EnsureAdminCredential(&auth.EnsureAdminCredentialRequest{
		UserId:     "admin-1",
		Identifier: "amin",
		Password:   "admin-secret-pw",
	})
	if err != nil {
		t.Fatalf("EnsureAdminCredential: %v", err)
	}
	if !resp.GetCreated() {
		t.Fatal("first ensure should report credential created")
	}
	row := creds.rows["admin-1"]
	if row == nil {
		t.Fatal("credential row not written")
	}
	if row.algo != passwordAlgoDBBcrypt {
		t.Fatalf("password_algo = %d, want %d (bcrypt)", row.algo, passwordAlgoDBBcrypt)
	}
	if bcrypt.CompareHashAndPassword([]byte(row.hash), []byte("admin-secret-pw")) != nil {
		t.Fatal("stored hash does not verify the requested password")
	}
}

func TestEnsureAdminCredentialDoesNotRotateExistingPassword(t *testing.T) {
	creds := newFakeCredentialsModel()
	guard := &fakeAccountsGuard{types: map[string]int64{"admin-1": model.AccountTypeDBAdmin}}
	logic := NewEnsureAdminCredentialLogic(context.Background(), newEnsureTestSvc(creds, guard))

	if _, err := logic.EnsureAdminCredential(&auth.EnsureAdminCredentialRequest{UserId: "admin-1", Password: "first-password"}); err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	firstHash := creds.rows["admin-1"].hash
	resp, err := logic.EnsureAdminCredential(&auth.EnsureAdminCredentialRequest{UserId: "admin-1", Password: "second-password"})
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if resp.GetCreated() {
		t.Fatal("second ensure should not report credential created")
	}
	if creds.rows["admin-1"].hash != firstHash {
		t.Fatal("admin ensure must not rotate an existing password")
	}
	if bcrypt.CompareHashAndPassword([]byte(creds.rows["admin-1"].hash), []byte("first-password")) != nil {
		t.Fatal("existing hash no longer verifies the original password")
	}
}

func TestEnsureAdminCredentialRejectsNonAdminAccount(t *testing.T) {
	creds := newFakeCredentialsModel()
	guard := &fakeAccountsGuard{types: map[string]int64{"test-1": model.AccountTypeDBTest, "user-1": 1}}
	logic := NewEnsureAdminCredentialLogic(context.Background(), newEnsureTestSvc(creds, guard))

	for _, id := range []string{"test-1", "user-1"} {
		if _, err := logic.EnsureAdminCredential(&auth.EnsureAdminCredentialRequest{UserId: id, Password: "whatever-pw"}); err == nil {
			t.Fatalf("ensure for non-admin account %q should fail", id)
		}
	}
	if len(creds.rows) != 0 {
		t.Fatal("no credential should be written for non-admin accounts")
	}
}
