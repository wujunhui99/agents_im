package readreceipt

import "errors"

var (
	ErrNegativeSeq       = errors.New("readreceipt: seq must be non-negative")
	ErrReadSeqExceedsMax = errors.New("readreceipt: read seq exceeds max seq")
)

type MarkReadResult struct {
	HasReadSeq  int64
	MaxSeq      int64
	UnreadCount int64
	Updated     bool
}

func NormalizeMarkRead(currentHasReadSeq, requestedHasReadSeq, maxSeq int64) (MarkReadResult, error) {
	if currentHasReadSeq < 0 || requestedHasReadSeq < 0 || maxSeq < 0 {
		return MarkReadResult{}, ErrNegativeSeq
	}
	if currentHasReadSeq > maxSeq || requestedHasReadSeq > maxSeq {
		return MarkReadResult{}, ErrReadSeqExceedsMax
	}

	nextHasReadSeq := currentHasReadSeq
	updated := CanAdvanceReadSeq(currentHasReadSeq, requestedHasReadSeq, maxSeq)
	if updated {
		nextHasReadSeq = requestedHasReadSeq
	}

	return MarkReadResult{
		HasReadSeq:  nextHasReadSeq,
		MaxSeq:      maxSeq,
		UnreadCount: UnreadCount(maxSeq, nextHasReadSeq),
		Updated:     updated,
	}, nil
}

func CanAdvanceReadSeq(currentHasReadSeq, requestedHasReadSeq, maxSeq int64) bool {
	if currentHasReadSeq < 0 || requestedHasReadSeq < 0 || maxSeq < 0 {
		return false
	}
	if currentHasReadSeq > maxSeq || requestedHasReadSeq > maxSeq {
		return false
	}
	return requestedHasReadSeq > currentHasReadSeq
}

func UnreadCount(maxSeq, hasReadSeq int64) int64 {
	if maxSeq < 0 {
		maxSeq = 0
	}
	if hasReadSeq < 0 {
		hasReadSeq = 0
	}
	if hasReadSeq >= maxSeq {
		return 0
	}
	return maxSeq - hasReadSeq
}
