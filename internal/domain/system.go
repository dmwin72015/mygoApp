package domain

import "context"

// Settings stores user and system preferences as key/value pairs.
type Settings interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

// Stats holds high-level business metrics for dashboards and reports.
type Stats struct {
	Students    int
	Teachers    int
	Courses     int
	Lessons     int
	Revenue     int64 // cents, sum of top-ups
	Outstanding int64 // cents, sum of student balances
}
