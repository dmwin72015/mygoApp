package domain

import "time"

// Classroom represents a physical or virtual room where lessons take place.
type Classroom struct {
	ID       int64  `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Capacity int    `json:"capacity"`
	Location string `json:"location"`
	Note     string `json:"note"`
}

// SetIdentity binds the numeric ID or business code.
func (c *Classroom) SetIdentity(rawID string) {
	SetEntityIdentity(&c.ID, &c.Code, rawID)
}

// Course represents a package of lessons sold at a specified price.
type Course struct {
	ID           int64  `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Subject      string `json:"subject"`
	TeacherID    int64  `json:"teacherId"`
	TotalLessons int    `json:"totalLessons"`
	PriceCents   int64  `json:"priceCents"`
	Minutes      int    `json:"minutes"`
	Active       bool   `json:"active"`
}

// SetIdentity binds the numeric ID or business code.
func (c *Course) SetIdentity(rawID string) {
	SetEntityIdentity(&c.ID, &c.Code, rawID)
}

// Lesson represents a single scheduled or conducted lesson session.
type Lesson struct {
	ID          int64     `json:"id"`
	Code        string    `json:"code"`
	CourseID    int64     `json:"courseId"`
	TeacherID   int64     `json:"teacherId"`
	ClassroomID int64     `json:"classroomId"`
	StartAt     time.Time `json:"startAt"`
	Minutes     int       `json:"minutes"`
	Status      string    `json:"status"` // planned | done | canceled
}

// SetIdentity binds the numeric ID or business code.
func (l *Lesson) SetIdentity(rawID string) {
	SetEntityIdentity(&l.ID, &l.Code, rawID)
}
