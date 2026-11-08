package domain

import (
	"context"
	"time"
)

// Ledger entry kinds.
const (
	KindTopUp   = "topup"   // 充值
	KindConsume = "consume" // 消课
)

// LedgerEntry records a balance mutation (top-up or lesson consumption).
type LedgerEntry struct {
	ID        string    `json:"id"`
	StudentID string    `json:"studentId"`
	Kind      string    `json:"kind"`
	Amount    int64     `json:"amount"` // cents, positive for top-up, negative for consumption
	Balance   int64     `json:"balance"`
	Note      string    `json:"note"`
	At        time.Time `json:"at"`
}

// Ledger provides append-only accounting operations.
type Ledger interface {
	List(ctx context.Context, studentID string, limit int) ([]LedgerEntry, error)
	TopUp(ctx context.Context, studentID string, amountCents int64, note string) (LedgerEntry, error)
	Consume(ctx context.Context, studentID string, amountCents int64, note string) (LedgerEntry, error)
}
