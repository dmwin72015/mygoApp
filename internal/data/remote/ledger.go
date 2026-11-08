package remote

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"mygo-app/internal/domain"
)

// remoteLedger manages billing, student balances, top-ups, and consumption over REST at /ledger.
type remoteLedger struct {
	c *Client
}

func newLedger(c *Client) domain.Ledger {
	return remoteLedger{c: c}
}

func (l remoteLedger) List(ctx context.Context, studentID any, limit int) ([]domain.LedgerEntry, error) {
	path := "/ledger"
	v := url.Values{}
	if studentID != nil && studentID != "" && studentID != int64(0) {
		v.Set("studentId", fmt.Sprint(studentID))
	}
	if limit > 0 {
		v.Set("limit", strconv.Itoa(limit))
	}
	if len(v) > 0 {
		path += "?" + v.Encode()
	}
	var resp struct {
		Items []domain.LedgerEntry `json:"items"`
	}
	if err := l.c.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (l remoteLedger) TopUp(ctx context.Context, studentID any, amountCents int64, note string) (domain.LedgerEntry, error) {
	return l.move(ctx, "topup", studentID, amountCents, note)
}

func (l remoteLedger) Consume(ctx context.Context, studentID any, amountCents int64, note string) (domain.LedgerEntry, error) {
	return l.move(ctx, "consume", studentID, amountCents, note)
}

func (l remoteLedger) move(ctx context.Context, kind string, studentID any, amountCents int64, note string) (domain.LedgerEntry, error) {
	var e domain.LedgerEntry
	err := l.c.Do(ctx, http.MethodPost, "/ledger/"+kind, map[string]any{
		"studentId": fmt.Sprint(studentID),
		"amount":    amountCents,
		"note":      note,
	}, &e)
	return e, err
}
