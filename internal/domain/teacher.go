package domain

import "time"

// Teacher represents an instructor who delivers lessons.
type Teacher struct {
	ID        int64     `json:"id" db:"id"`
	Code      string    `json:"code" db:"code"`
	Name      string    `json:"name" db:"name"`
	Phone     string    `json:"phone" db:"phone"`
	Subject   string    `json:"subject" db:"subject"`
	RateCents int64     `json:"rateCents" db:"rate_cents"` // cents per lesson
	Active    bool      `json:"active" db:"active"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

// SetIdentity binds the numeric ID or business code.
func (t *Teacher) SetIdentity(rawID string) {
	SetEntityIdentity(&t.ID, &t.Code, rawID)
}
