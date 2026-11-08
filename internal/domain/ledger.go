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
	ID        int64     `json:"id" db:"id"`
	Code      string    `json:"code" db:"code"`
	StudentID int64     `json:"studentId" db:"student_id"`
	Kind      string    `json:"kind" db:"kind"`
	Amount    int64     `json:"amount" db:"amount_cents"` // cents, positive for top-up, negative for consumption
	Balance   int64     `json:"balance" db:"balance_cents"`
	Note      string    `json:"note" db:"note"`
	At        time.Time `json:"at" db:"at"`
}

// Ledger provides append-only accounting operations.
type Ledger interface {
	List(ctx context.Context, studentID any, limit int) ([]LedgerEntry, error)
	TopUp(ctx context.Context, studentID any, amountCents int64, note string) (LedgerEntry, error)
	Consume(ctx context.Context, studentID any, amountCents int64, note string) (LedgerEntry, error)
}
