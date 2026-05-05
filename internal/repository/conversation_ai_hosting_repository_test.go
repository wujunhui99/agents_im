package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
)

func TestMemoryConversationAIHostingRepositoryConcurrentEnableLeavesOneOwnerEnabled(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryConversationAIHostingRepository()
	conversationID := SingleConversationID("usr_a", "usr_b")

	var wg sync.WaitGroup
	for _, ownerID := range []string{"usr_a", "usr_b"} {
		ownerID := ownerID
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.SetConversationAIHostingEnabled(ctx, ConversationAIHostingUpdate{
				OwnerAccountID:    ownerID,
				ConversationID:    conversationID,
				Enabled:           true,
				MaxRecentMessages: 30,
			})
			if err != nil && apperror.From(err).Code != apperror.CodeAlreadyExists {
				t.Errorf("enable %s: %v", ownerID, err)
			}
		}()
	}
	wg.Wait()

	enabled, err := repo.GetEnabledConversationAIHosting(ctx, conversationID)
	if err != nil {
		t.Fatalf("get enabled hosting: %v", err)
	}
	if !enabled.Enabled {
		t.Fatalf("enabled setting not marked enabled: %+v", enabled)
	}
	if enabled.OwnerAccountID != "usr_a" && enabled.OwnerAccountID != "usr_b" {
		t.Fatalf("unexpected enabled owner: %+v", enabled)
	}

	peerID := "usr_a"
	if enabled.OwnerAccountID == "usr_a" {
		peerID = "usr_b"
	}
	_, err = repo.SetConversationAIHostingEnabled(ctx, ConversationAIHostingUpdate{
		OwnerAccountID:    peerID,
		ConversationID:    conversationID,
		Enabled:           true,
		MaxRecentMessages: 30,
	})
	if apperror.From(err).Code != apperror.CodeAlreadyExists {
		t.Fatalf("peer enable after winner error = %v, want conflict", err)
	}
}
