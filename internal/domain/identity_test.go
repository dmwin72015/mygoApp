package domain_test

import (
	"testing"

	"mygo-app/internal/domain"
)

func TestIdentifiable(t *testing.T) {
	tests := []struct {
		name     string
		item     domain.Identifiable
		rawID    string
		wantID   int64
		wantCode string
	}{
		{
			name:     "Student numeric ID",
			item:     &domain.Student{},
			rawID:    "123",
			wantID:   123,
			wantCode: "",
		},
		{
			name:     "Student string Code",
			item:     &domain.Student{},
			rawID:    "STU-001",
			wantID:   0,
			wantCode: "STU-001",
		},
		{
			name:     "Teacher numeric ID",
			item:     &domain.Teacher{},
			rawID:    "456",
			wantID:   456,
			wantCode: "",
		},
		{
			name:     "Teacher string Code",
			item:     &domain.Teacher{},
			rawID:    "TEA-002",
			wantID:   0,
			wantCode: "TEA-002",
		},
		{
			name:     "Classroom numeric ID",
			item:     &domain.Classroom{},
			rawID:    "789",
			wantID:   789,
			wantCode: "",
		},
		{
			name:     "Classroom string Code",
			item:     &domain.Classroom{},
			rawID:    "ROOM-301",
			wantID:   0,
			wantCode: "ROOM-301",
		},
		{
			name:     "Course numeric ID",
			item:     &domain.Course{},
			rawID:    "101",
			wantID:   101,
			wantCode: "",
		},
		{
			name:     "Course string Code",
			item:     &domain.Course{},
			rawID:    "MATH-101",
			wantID:   0,
			wantCode: "MATH-101",
		},
		{
			name:     "Lesson numeric ID",
			item:     &domain.Lesson{},
			rawID:    "202",
			wantID:   202,
			wantCode: "",
		},
		{
			name:     "Lesson string Code",
			item:     &domain.Lesson{},
			rawID:    "LESSON-A",
			wantID:   0,
			wantCode: "LESSON-A",
		},
		{
			name:     "User numeric ID",
			item:     &domain.User{},
			rawID:    "999",
			wantID:   999,
			wantCode: "",
		},
		{
			name:     "User string Code",
			item:     &domain.User{},
			rawID:    "admin-root",
			wantID:   0,
			wantCode: "admin-root",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.item.SetIdentity(tc.rawID)
			switch v := tc.item.(type) {
			case *domain.Student:
				if v.ID != tc.wantID || v.Code != tc.wantCode {
					t.Errorf("got ID=%d Code=%q, want ID=%d Code=%q", v.ID, v.Code, tc.wantID, tc.wantCode)
				}
			case *domain.Teacher:
				if v.ID != tc.wantID || v.Code != tc.wantCode {
					t.Errorf("got ID=%d Code=%q, want ID=%d Code=%q", v.ID, v.Code, tc.wantID, tc.wantCode)
				}
			case *domain.Classroom:
				if v.ID != tc.wantID || v.Code != tc.wantCode {
					t.Errorf("got ID=%d Code=%q, want ID=%d Code=%q", v.ID, v.Code, tc.wantID, tc.wantCode)
				}
			case *domain.Course:
				if v.ID != tc.wantID || v.Code != tc.wantCode {
					t.Errorf("got ID=%d Code=%q, want ID=%d Code=%q", v.ID, v.Code, tc.wantID, tc.wantCode)
				}
			case *domain.Lesson:
				if v.ID != tc.wantID || v.Code != tc.wantCode {
					t.Errorf("got ID=%d Code=%q, want ID=%d Code=%q", v.ID, v.Code, tc.wantID, tc.wantCode)
				}
			case *domain.User:
				if v.ID != tc.wantID || v.Code != tc.wantCode {
					t.Errorf("got ID=%d Code=%q, want ID=%d Code=%q", v.ID, v.Code, tc.wantID, tc.wantCode)
				}
			}
		})
	}
}
