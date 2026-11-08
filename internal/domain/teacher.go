package domain

import "time"

// Teacher represents an instructor who delivers lessons.
type Teacher struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Subject   string    `json:"subject"`
	RateCents int64     `json:"rateCents"` // cents per lesson
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
}
