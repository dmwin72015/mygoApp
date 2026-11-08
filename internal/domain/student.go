package domain

import "time"

// Student represents a student with prepaid tuition balance.
type Student struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Guardian  string    `json:"guardian"`
	Phone     string    `json:"phone"`
	Balance   int64     `json:"balance"` // cents
	Note      string    `json:"note"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
}
