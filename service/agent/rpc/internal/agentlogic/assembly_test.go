package agentlogic

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/registry"
)

// fakeAccountPort 是 AccountPort 测试桩：内存账号表，Create 分配 acc_<seq> 主键。
type fakeAccountPort struct {
	mu           sync.Mutex
	seq          int
	byID         map[string]model.User
	byIdentifier map[string]string
}

func newFakeAccountPort() *fakeAccountPort {
	return &fakeAccountPort{byID: map[string]model.User{}, byIdentifier: map[string]string{}}
}

func (f *fakeAccountPort) seed(account model.User) {
	f.byID[account.AccountID] = account
	if account.Identifier != "" {
		f.byIdentifier[account.Identifier] = account.AccountID
	}
}

func (f *fakeAccountPort) GetByID(_ context.Context, accountID string) (model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[accountID]
	if !ok {
		return model.User{}, apperror.NotFound("account not found")
	}
	return u, nil
}

func (f *fakeAccountPort) ExistsByIdentifier(_ context.Context, identifier string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.byIdentifier[identifier]
	return ok, nil
}

func (f *fakeAccountPort) Create(_ context.Context, account model.User) (model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byIdentifier[account.Identifier]; ok {
		return model.User{}, apperror.AlreadyExists("identifier already exists")
	}
	f.seq++
	account.AccountID = "acc_" + strconv.Itoa(f.seq)
	account.UserID = account.AccountID
	f.byID[account.AccountID] = account
	f.byIdentifier[account.Identifier] = account.AccountID
	return account, nil
}

// fakeFriendPort 记录 EnsureFriendship 调用，验证 saga 末尾建好友。
type fakeFriendPort struct {
	calls [][2]string
}

func (f *fakeFriendPort) EnsureFriendship(_ context.Context, userID, friendID string) error {
	f.calls = append(f.calls, [2]string{userID, friendID})
	return nil
}

func TestAgentAssemblyCreateAgentFromToolHappyPath(t *testing.T) {
	ctx := context.Background()

	accounts := newFakeAccountPort()
	accounts.seed(model.User{AccountID: "acc_creator", Identifier: DefaultAssistantIdentifier, AccountType: model.AccountTypeAgent})
	accounts.seed(model.User{AccountID: "usr_1", Identifier: "alice", AccountType: model.AccountTypeUser})

	agents := NewMemoryAgentStore()
	creator, err := agents.CreateAgent(ctx, model.Agent{
		AccountID: "acc_creator",
		Name:      DefaultAssistantAgentName,
		Status:    model.AgentStatusActive,
		CreatedBy: "acc_creator",
	})
	if err != nil {
		t.Fatalf("seed creator agent: %v", err)
	}

	reg := registry.NewMemoryStore()
	if _, err := reg.RegisterTool(ctx, model.AgentTool{
		Name:            model.BuiltinToolReadConversationContext,
		ToolType:        model.AgentToolTypeBuiltin,
		BuiltinKey:      model.BuiltinToolReadConversationContext,
		Status:          model.AgentToolStatusActive,
		AdminConfigured: true,
		CreatedBy:       "acc_creator",
	}); err != nil {
		t.Fatalf("seed builtin tool: %v", err)
	}

	friends := &fakeFriendPort{}
	assembly := NewAgentAssemblyLogic(AgentAssemblyDependencies{
		Accounts:    accounts,
		Friendships: friends,
		Agents:      agents,
		Registry:    reg,
	})

	resp, err := assembly.CreateAgentFromTool(ctx, AgentCreateToolRequest{
		CreatorAgentID:   creator.AgentID,
		RequestingUserID: "usr_1",
		Name:             "Helper Bot",
		Description:      "helps users with tasks",
		ToolNames:        []string{model.BuiltinToolReadConversationContext},
	})
	if err != nil {
		t.Fatalf("create agent from tool: %v", err)
	}
	if resp.AgentID == "" || resp.AccountID == "" || resp.PromptID == "" {
		t.Fatalf("incomplete create response: %+v", resp)
	}
	if resp.FriendUserID != "usr_1" {
		t.Fatalf("friend user = %q, want usr_1", resp.FriendUserID)
	}
	if len(resp.ToolNames) != 1 || resp.ToolNames[0] != model.BuiltinToolReadConversationContext {
		t.Fatalf("bound tool names = %v", resp.ToolNames)
	}

	// 新建 agent 落在 agent 域数据层，状态 active。
	created, err := agents.GetAgent(ctx, resp.AgentID)
	if err != nil || created.Status != model.AgentStatusActive {
		t.Fatalf("created agent = %+v err=%v", created, err)
	}
	// saga 末尾经 friends-rpc 端口互为好友（requester ↔ new agent account）。
	if len(friends.calls) != 1 || friends.calls[0] != [2]string{"usr_1", resp.AccountID} {
		t.Fatalf("EnsureFriendship calls = %v", friends.calls)
	}
}

func TestAgentAssemblyCreateAgentFromToolRejectsNonAssistantCaller(t *testing.T) {
	ctx := context.Background()

	accounts := newFakeAccountPort()
	// caller agent account is an agent but NOT the default assistant identifier.
	accounts.seed(model.User{AccountID: "acc_other", Identifier: "other_agent", AccountType: model.AccountTypeAgent})

	agents := NewMemoryAgentStore()
	caller, err := agents.CreateAgent(ctx, model.Agent{
		AccountID: "acc_other",
		Name:      "Other",
		Status:    model.AgentStatusActive,
		CreatedBy: "acc_other",
	})
	if err != nil {
		t.Fatalf("seed caller agent: %v", err)
	}

	assembly := NewAgentAssemblyLogic(AgentAssemblyDependencies{
		Accounts:    accounts,
		Friendships: &fakeFriendPort{},
		Agents:      agents,
		Registry:    registry.NewMemoryStore(),
	})

	_, err = assembly.CreateAgentFromTool(ctx, AgentCreateToolRequest{
		CreatorAgentID:   caller.AgentID,
		RequestingUserID: "usr_1",
		Name:             "Helper Bot",
		Description:      "helps",
	})
	if apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("non-assistant caller error = %v, want FORBIDDEN", err)
	}
}
