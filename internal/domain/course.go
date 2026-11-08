package domain

import "time"

// Classroom represents a physical or virtual room where lessons take place.
type Classroom struct {
	ID       int64  `json:"id" db:"id"`
	Code     string `json:"code" db:"code"`
	Name     string `json:"name" db:"name"`
	Capacity int    `json:"capacity" db:"capacity"`
	Location string `json:"location" db:"location"`
	Note     string `json:"note" db:"note"`
}

// SetIdentity binds the numeric ID or business code.
func (c *Classroom) SetIdentity(rawID string) {
	SetEntityIdentity(&c.ID, &c.Code, rawID)
}

// Course represents a package of lessons sold at a specified price.
type Course struct {
	ID           int64  `json:"id" db:"id"`
	Code         string `json:"code" db:"code"`
	Name         string `json:"name" db:"name"`
	Subject      string `json:"subject" db:"subject"`
	TeacherID    int64  `json:"teacherId" db:"teacher_id"`
	TotalLessons int    `json:"totalLessons" db:"total_lessons"`
	PriceCents   int64  `json:"priceCents" db:"price_cents"`
	Minutes      int    `json:"minutes" db:"minutes"`
	Active       bool   `json:"active" db:"active"`
}

// SetIdentity binds the numeric ID or business code.
func (c *Course) SetIdentity(rawID string) {
	SetEntityIdentity(&c.ID, &c.Code, rawID)
}

// Lesson represents a single scheduled or conducted lesson session.
type Lesson struct {
	ID          int64     `json:"id" db:"id"`
	Code        string    `json:"code" db:"code"`
	CourseID    int64     `json:"courseId" db:"course_id"`
	TeacherID   int64     `json:"teacherId" db:"teacher_id"`
	ClassroomID int64     `json:"classroomId" db:"classroom_id"`
	StartAt     time.Time `json:"startAt" db:"start_at"`
	Minutes     int       `json:"minutes" db:"minutes"`
	Status      string    `json:"status" db:"status"` // planned | done | canceled
}

// SetIdentity binds the numeric ID or business code.
func (l *Lesson) SetIdentity(rawID string) {
	SetEntityIdentity(&l.ID, &l.Code, rawID)
}
