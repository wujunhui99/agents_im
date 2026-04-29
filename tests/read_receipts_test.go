package tests

import (
	"errors"
	"testing"

	"github.com/wujunhui99/agents_im/internal/domain/readreceipt"
)

func TestReadReceiptsUnreadCount(t *testing.T) {
	tests := []struct {
		name       string
		maxSeq     int64
		hasReadSeq int64
		want       int64
	}{
		{name: "empty conversation", maxSeq: 0, hasReadSeq: 0, want: 0},
		{name: "nothing read", maxSeq: 10, hasReadSeq: 0, want: 10},
		{name: "partially read", maxSeq: 10, hasReadSeq: 7, want: 3},
		{name: "fully read", maxSeq: 10, hasReadSeq: 10, want: 0},
		{name: "invalid over-read state never negative", maxSeq: 10, hasReadSeq: 12, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := readreceipt.UnreadCount(tt.maxSeq, tt.hasReadSeq); got != tt.want {
				t.Fatalf("UnreadCount(%d, %d) = %d, want %d", tt.maxSeq, tt.hasReadSeq, got, tt.want)
			}
		})
	}
}

func TestReadReceiptsNormalizeMarkReadAdvancesMonotonically(t *testing.T) {
	result, err := readreceipt.NormalizeMarkRead(2, 5, 8)
	if err != nil {
		t.Fatalf("NormalizeMarkRead: %v", err)
	}
	if !result.Updated {
		t.Fatal("mark read should report updated")
	}
	if result.HasReadSeq != 5 {
		t.Fatalf("HasReadSeq = %d, want 5", result.HasReadSeq)
	}
	if result.UnreadCount != 3 {
		t.Fatalf("UnreadCount = %d, want 3", result.UnreadCount)
	}
	if !readreceipt.CanAdvanceReadSeq(2, 5, 8) {
		t.Fatal("CanAdvanceReadSeq should allow a valid forward move")
	}
}

func TestReadReceiptsDuplicateMarkReadIsIdempotent(t *testing.T) {
	result, err := readreceipt.NormalizeMarkRead(5, 5, 8)
	if err != nil {
		t.Fatalf("NormalizeMarkRead: %v", err)
	}
	if result.Updated {
		t.Fatal("duplicate mark read should not report updated")
	}
	if result.HasReadSeq != 5 {
		t.Fatalf("HasReadSeq = %d, want 5", result.HasReadSeq)
	}
	if result.UnreadCount != 3 {
		t.Fatalf("UnreadCount = %d, want 3", result.UnreadCount)
	}
}

func TestReadReceiptsRollbackMarkReadDoesNotTakeEffect(t *testing.T) {
	result, err := readreceipt.NormalizeMarkRead(7, 3, 9)
	if err != nil {
		t.Fatalf("NormalizeMarkRead: %v", err)
	}
	if result.Updated {
		t.Fatal("rollback mark read should not report updated")
	}
	if result.HasReadSeq != 7 {
		t.Fatalf("HasReadSeq = %d, want 7", result.HasReadSeq)
	}
	if result.UnreadCount != 2 {
		t.Fatalf("UnreadCount = %d, want 2", result.UnreadCount)
	}
	if readreceipt.CanAdvanceReadSeq(7, 3, 9) {
		t.Fatal("CanAdvanceReadSeq should reject a rollback")
	}
}

func TestReadReceiptsMarkReadRejectsSeqBeyondMax(t *testing.T) {
	_, err := readreceipt.NormalizeMarkRead(3, 11, 10)
	if !errors.Is(err, readreceipt.ErrReadSeqExceedsMax) {
		t.Fatalf("NormalizeMarkRead error = %v, want ErrReadSeqExceedsMax", err)
	}
	if readreceipt.CanAdvanceReadSeq(3, 11, 10) {
		t.Fatal("CanAdvanceReadSeq should reject a request beyond max_seq")
	}
}
