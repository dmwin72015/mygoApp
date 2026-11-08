package domain

import "time"

// Teacher represents an instructor who delivers lessons.
type Teacher struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Subject   string    `json:"subject"`
	RateCents int64     `json:"rateCents"` // cents per lesson
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
}

// SetIdentity binds the numeric ID or business code.
func (t *Teacher) SetIdentity(rawID string) {
	SetEntityIdentity(&t.ID, &t.Code, rawID)
}
