package domain

import "time"

// Student represents a student with prepaid tuition balance.
type Student struct {
	ID        int64     `json:"id" db:"id"`
	Code      string    `json:"code" db:"code"`
	Name      string    `json:"name" db:"name"`
	Guardian  string    `json:"guardian" db:"guardian"`
	Phone     string    `json:"phone" db:"phone"`
	Balance   int64     `json:"balance" db:"balance_cents"` // cents
	Note      string    `json:"note" db:"note"`
	Active    bool      `json:"active" db:"active"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

// SetIdentity binds the numeric ID or business code.
func (s *Student) SetIdentity(rawID string) {
	SetEntityIdentity(&s.ID, &s.Code, rawID)
}
