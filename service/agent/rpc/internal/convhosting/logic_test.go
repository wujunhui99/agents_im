package convhosting

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

func singleConv(a, b string) string {
	users := []string{a, b}
	sort.Strings(users)
	return "single:" + users[0] + ":" + users[1]
}

type agentAccountExistenceCheckerFunc func(ctx context.Context, accountID string) (bool, error)

func (f agentAccountExistenceCheckerFunc) IsActiveAgentAccount(ctx context.Context, accountID string) (bool, error) {
	return f(ctx, accountID)
}

func TestConversationAIHostingDefaultEnableDisableAndPeerConflict(t *testing.T) {
	ctx := context.Background()
	hosting := NewConversationAIHostingLogic(NewMemoryStore())
	conversationID := singleConv("usr_a", "usr_b")

	initial, err := hosting.GetConversationAIHosting(ctx, GetConversationAIHostingRequest{
		OwnerAccountID: "usr_a",
		ConversationID: conversationID,
	})
	if err != nil {
		t.Fatalf("get default hosting: %v", err)
	}
	if initial.Enabled || initial.PeerEnabled || !initial.Available {
		t.Fatalf("default hosting state mismatch: %+v", initial)
	}

	enabled, err := hosting.UpdateConversationAIHosting(ctx, UpdateConversationAIHostingRequest{
		OwnerAccountID: "usr_a",
		ConversationID: conversationID,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("enable hosting: %v", err)
	}
	if !enabled.Enabled || enabled.PeerEnabled || !enabled.Available {
		t.Fatalf("enabled hosting state mismatch: %+v", enabled)
	}

	peerView, err := hosting.GetConversationAIHosting(ctx, GetConversationAIHostingRequest{
		OwnerAccountID: "usr_b",
		ConversationID: conversationID,
	})
	if err != nil {
		t.Fatalf("get peer hosting view: %v", err)
	}
	if peerView.Enabled || !peerView.PeerEnabled || peerView.Available {
		t.Fatalf("peer view did not expose unavailable conflict: %+v", peerView)
	}
	if !strings.Contains(peerView.UnavailableReason, "对方已开启") {
		t.Fatalf("peer unavailable reason must be Chinese and clear, got %q", peerView.UnavailableReason)
	}

	_, err = hosting.UpdateConversationAIHosting(ctx, UpdateConversationAIHostingRequest{
		OwnerAccountID: "usr_b",
		ConversationID: conversationID,
		Enabled:        true,
	})
	if apperror.From(err).Code != apperror.CodeAlreadyExists {
		t.Fatalf("peer enable error = %v, want conflict", err)
	}

	disabled, err := hosting.UpdateConversationAIHosting(ctx, UpdateConversationAIHostingRequest{
		OwnerAccountID: "usr_a",
		ConversationID: conversationID,
		Enabled:        false,
	})
	if err != nil {
		t.Fatalf("disable hosting: %v", err)
	}
	if disabled.Enabled || disabled.PeerEnabled || !disabled.Available {
		t.Fatalf("disabled hosting state mismatch: %+v", disabled)
	}

	peerEnabled, err := hosting.UpdateConversationAIHosting(ctx, UpdateConversationAIHostingRequest{
		OwnerAccountID: "usr_b",
		ConversationID: conversationID,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("peer enable after disable: %v", err)
	}
	if !peerEnabled.Enabled || peerEnabled.PeerEnabled || !peerEnabled.Available {
		t.Fatalf("peer enabled state mismatch: %+v", peerEnabled)
	}
}

func TestConversationAIHostingRejectsNonParticipantsAndGroups(t *testing.T) {
	ctx := context.Background()
	hosting := NewConversationAIHostingLogic(NewMemoryStore())

	_, err := hosting.GetConversationAIHosting(ctx, GetConversationAIHostingRequest{
		OwnerAccountID: "usr_outsider",
		ConversationID: singleConv("usr_a", "usr_b"),
	})
	if apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("non-participant read error = %v, want forbidden", err)
	}

	_, err = hosting.UpdateConversationAIHosting(ctx, UpdateConversationAIHostingRequest{
		OwnerAccountID: "usr_outsider",
		ConversationID: singleConv("usr_a", "usr_b"),
		Enabled:        true,
	})
	if apperror.From(err).Code != apperror.CodeForbidden {
		t.Fatalf("non-participant update error = %v, want forbidden", err)
	}

	_, err = hosting.GetConversationAIHosting(ctx, GetConversationAIHostingRequest{
		OwnerAccountID: "usr_a",
		ConversationID: "group:grp_1",
	})
	if apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("group read error = %v, want invalid argument", err)
	}
}

func TestConversationAIHostingRejectsAgentConversation(t *testing.T) {
	ctx := context.Background()
	hosting := NewConversationAIHostingLogic(NewMemoryStore()).WithAgentAccountResolver(agentAccountExistenceCheckerFunc(func(_ context.Context, accountID string) (bool, error) {
		return accountID == "agent_creator", nil
	}))
	conversationID := singleConv("usr_new", "agent_creator")

	status, err := hosting.GetConversationAIHosting(ctx, GetConversationAIHostingRequest{
		OwnerAccountID: "usr_new",
		ConversationID: conversationID,
	})
	if err != nil {
		t.Fatalf("agent conversation status should be visible as unavailable, got: %v", err)
	}
	if status.Available || status.Enabled || status.PeerEnabled {
		t.Fatalf("agent conversation hosting status = %+v, want unavailable and disabled", status)
	}
	if !strings.Contains(status.UnavailableReason, "AI 助手") {
		t.Fatalf("agent conversation unavailable reason = %q, want AI assistant reason", status.UnavailableReason)
	}

	_, err = hosting.UpdateConversationAIHosting(ctx, UpdateConversationAIHostingRequest{
		OwnerAccountID: "usr_new",
		ConversationID: conversationID,
		Enabled:        true,
	})
	if apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("agent conversation enable error = %v, want invalid argument", err)
	}

	agentOwnerStatus, err := hosting.GetConversationAIHosting(ctx, GetConversationAIHostingRequest{
		OwnerAccountID: "agent_creator",
		ConversationID: conversationID,
	})
	if err != nil {
		t.Fatalf("agent-owned conversation status should be visible as unavailable, got: %v", err)
	}
	if agentOwnerStatus.Available {
		t.Fatalf("agent-owned conversation hosting status = %+v, want unavailable", agentOwnerStatus)
	}
	_, err = hosting.UpdateConversationAIHosting(ctx, UpdateConversationAIHostingRequest{
		OwnerAccountID: "agent_creator",
		ConversationID: conversationID,
		Enabled:        true,
	})
	if apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("agent-owned conversation enable error = %v, want invalid argument", err)
	}
}
