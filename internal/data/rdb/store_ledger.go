package rdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"mygo-app/internal/domain"
)

// StudentBalanceService abstracts operations on students required by Ledger.
type StudentBalanceService interface {
	ResolveID(ctx context.Context, studentID any) (int64, error)
	LockAndGetBalance(ctx context.Context, id int64) (int64, error)
	UpdateBalance(ctx context.Context, id int64, newBalance int64) error
}

// LedgerStore manages financial ledger transactions across relational databases.
type LedgerStore struct {
	db       DBTX
	dialect  *Dialect
	txm      TxManager
	students StudentBalanceService
}

func NewLedgerStore(db DBTX, dialect *Dialect, txm TxManager, students StudentBalanceService) *LedgerStore {
	return &LedgerStore{
		db:       db,
		dialect:  dialect,
		txm:      txm,
		students: students,
	}
}

func (l *LedgerStore) dbtx(ctx context.Context) DBTX {
	return GetDBTX(ctx, l.db)
}

func (l *LedgerStore) List(ctx context.Context, studentID any, limit int) ([]domain.LedgerEntry, error) {
	qb := NewQueryBuilder(l.dialect, "SELECT id, code, student_id, kind, amount_cents, balance_cents, note, at FROM ledger")
	if studentID != nil && studentID != "" && studentID != int64(0) {
		stuID, err := l.students.ResolveID(ctx, studentID)
		if err == nil {
			qb.Where("student_id = ?", stuID)
		}
	}
	qb.OrderBy("at DESC")
	qb.Limit(limit)

	query, args := qb.Build()
	var list []domain.LedgerEntry
	err := l.dbtx(ctx).SelectContext(ctx, &list, query, args...)
	if list == nil {
		list = []domain.LedgerEntry{}
	}
	return list, err
}

func (l *LedgerStore) TopUp(ctx context.Context, studentID any, amountCents int64, note string) (domain.LedgerEntry, error) {
	if amountCents <= 0 {
		return domain.LedgerEntry{}, errors.New("充值金额必须大于 0")
	}
	return l.apply(ctx, studentID, "topup", amountCents, note)
}

func (l *LedgerStore) Consume(ctx context.Context, studentID any, amountCents int64, note string) (domain.LedgerEntry, error) {
	if amountCents <= 0 {
		return domain.LedgerEntry{}, errors.New("扣费金额必须大于 0")
	}
	return l.apply(ctx, studentID, "consume", -amountCents, note)
}

func (l *LedgerStore) apply(ctx context.Context, studentID any, kind string, delta int64, note string) (domain.LedgerEntry, error) {
	var entry domain.LedgerEntry

	err := l.txm.RunInTxLevel(ctx, sql.LevelReadCommitted, func(txCtx context.Context) error {
		stuID, err := l.students.ResolveID(txCtx, studentID)
		if err != nil {
			return fmt.Errorf("student %v: %w", studentID, domain.ErrNotFound)
		}

		currentBalance, err := l.students.LockAndGetBalance(txCtx, stuID)
		if err != nil {
			return fmt.Errorf("student %v: %w", studentID, err)
		}

		newBalance := currentBalance + delta
		if err := l.students.UpdateBalance(txCtx, stuID, newBalance); err != nil {
			return err
		}

		entry = domain.LedgerEntry{
			Code:      domain.NewID("ldg"),
			StudentID: stuID,
			Kind:      kind,
			Amount:    delta,
			Balance:   newBalance,
			Note:      note,
			At:        time.Now(),
		}

		newID, err := l.dialect.InsertAndGetID(txCtx, l.dbtx(txCtx),
			`INSERT INTO ledger (code, student_id, kind, amount_cents, balance_cents, note, at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			entry.Code, entry.StudentID, entry.Kind, entry.Amount, entry.Balance, entry.Note, entry.At,
		)
		if err != nil {
			return err
		}
		entry.ID = newID
		return nil
	})

	if err != nil {
		return domain.LedgerEntry{}, err
	}
	return entry, nil
}
