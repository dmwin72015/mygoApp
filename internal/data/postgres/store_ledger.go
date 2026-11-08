package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"mygo-app/internal/domain"
)

type pgLedgerStore struct{ r *Repo }

func (l pgLedgerStore) resolveStudentID(ctx context.Context, q DBTX, studentID any) (int64, error) {
	switch v := studentID.(type) {
	case int64:
		if v > 0 {
			return v, nil
		}
	case int:
		if v > 0 {
			return int64(v), nil
		}
	case string:
		if num, err := strconv.ParseInt(v, 10, 64); err == nil && num > 0 {
			return num, nil
		}
		var realID int64
		err := q.QueryRowContext(ctx, "SELECT id FROM students WHERE code = $1", v).Scan(&realID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, domain.ErrNotFound
			}
			return 0, err
		}
		return realID, nil
	}
	return 0, domain.ErrNotFound
}

func (l pgLedgerStore) List(ctx context.Context, studentID any, limit int) ([]domain.LedgerEntry, error) {
	qb := NewQueryBuilder("SELECT id, code, student_id, kind, amount_cents, balance_cents, note, at FROM ledger")
	if studentID != nil && studentID != "" && studentID != int64(0) {
		stuID, err := l.resolveStudentID(ctx, l.r.db, studentID)
		if err == nil {
			qb.Where("student_id = ?", stuID)
		}
	}
	qb.OrderBy("at DESC")
	qb.Limit(limit)

	query, args := qb.Build()
	rows, err := l.r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.LedgerEntry
	for rows.Next() {
		var item domain.LedgerEntry
		if err := rows.Scan(&item.ID, &item.Code, &item.StudentID, &item.Kind, &item.Amount, &item.Balance, &item.Note, &item.At); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (l pgLedgerStore) TopUp(ctx context.Context, studentID any, amountCents int64, note string) (domain.LedgerEntry, error) {
	if amountCents <= 0 {
		return domain.LedgerEntry{}, errors.New("充值金额必须大于 0")
	}
	return l.apply(ctx, studentID, "topup", amountCents, note)
}

func (l pgLedgerStore) Consume(ctx context.Context, studentID any, amountCents int64, note string) (domain.LedgerEntry, error) {
	if amountCents <= 0 {
		return domain.LedgerEntry{}, errors.New("扣费金额必须大于 0")
	}
	return l.apply(ctx, studentID, "consume", -amountCents, note)
}

func (l pgLedgerStore) apply(ctx context.Context, studentID any, kind string, delta int64, note string) (domain.LedgerEntry, error) {
	var entry domain.LedgerEntry

	err := l.r.WithTxLevel(ctx, sql.LevelReadCommitted, func(tx *sql.Tx) error {
		stuID, err := l.resolveStudentID(ctx, tx, studentID)
		if err != nil {
			return fmt.Errorf("student %v: %w", studentID, domain.ErrNotFound)
		}

		var currentBalance int64
		err = tx.QueryRowContext(ctx, "SELECT balance_cents FROM students WHERE id = $1 FOR UPDATE", stuID).Scan(&currentBalance)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("student %v: %w", studentID, domain.ErrNotFound)
		}
		if err != nil {
			return err
		}

		newBalance := currentBalance + delta

		_, err = tx.ExecContext(ctx, "UPDATE students SET balance_cents = $1 WHERE id = $2", newBalance, stuID)
		if err != nil {
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

		return tx.QueryRowContext(ctx,
			`INSERT INTO ledger (code, student_id, kind, amount_cents, balance_cents, note, at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING id`,
			entry.Code, entry.StudentID, entry.Kind, entry.Amount, entry.Balance, entry.Note, entry.At,
		).Scan(&entry.ID)
	})

	if err != nil {
		return domain.LedgerEntry{}, err
	}
	return entry, nil
}
