package agentlogic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/agentlogictest"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
)

// fakeAccountReader 是 AccountReader 测试桩：按 accountID 返回预设账号类型。
type fakeAccountReader struct {
	accounts map[string]model.AccountType
}

func (f fakeAccountReader) GetByID(_ context.Context, accountID string) (model.User, error) {
	at, ok := f.accounts[accountID]
	if !ok {
		return model.User{}, apperror.NotFound("account not found")
	}
	return model.User{AccountID: accountID, AccountType: at}, nil
}

func (f fakeAccountReader) ExistsByIdentifier(context.Context, string) (bool, error) {
	return false, nil
}

func TestAgentLogicCreateRequiresAgentAccountType(t *testing.T) {
	ctx := context.Background()
	logic := NewAgentLogic(agentlogictest.NewMemoryAgentStore(), fakeAccountReader{accounts: map[string]model.AccountType{
		"usr_agent": model.AccountTypeAgent,
		"usr_user":  model.AccountTypeUser,
	}})

	created, err := logic.CreateAgent(ctx, CreateAgentRequest{
		IMUserID:    "usr_agent",
		Name:        "Support Bot",
		Description: "handles support triage",
		CreatedBy:   "usr_admin",
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if created.AgentID == "" || created.IMUserID != "usr_agent" || created.Status != AgentStatusDisabled {
		t.Fatalf("unexpected created agent: %+v", created)
	}

	if _, err := logic.CreateAgent(ctx, CreateAgentRequest{
		IMUserID:  "usr_user",
		Name:      "Wrong Type Bot",
		CreatedBy: "usr_admin",
	}); apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("user-type account binding error = %v, want FORBIDDEN", err)
	}
}

func TestAgentLogicUpdateListStatusFlow(t *testing.T) {
	ctx := context.Background()
	store := agentlogictest.NewMemoryAgentStore()
	logic := NewAgentLogic(store, fakeAccountReader{accounts: map[string]model.AccountType{"usr_agent": model.AccountTypeAgent}})

	created, err := logic.CreateAgent(ctx, CreateAgentRequest{IMUserID: "usr_agent", Name: "Bot", CreatedBy: "usr_admin"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	activated, err := logic.UpdateAgentStatus(ctx, UpdateAgentStatusRequest{AgentID: created.AgentID, Status: "active"})
	if err != nil || activated.Status != AgentStatusActive {
		t.Fatalf("activate status = %+v err=%v", activated, err)
	}
	got, err := logic.GetAgent(ctx, created.AgentID)
	if err != nil || got.AgentID != created.AgentID {
		t.Fatalf("get agent = %+v err=%v", got, err)
	}
}
