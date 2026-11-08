package server

import (
	"testing"

	"mygo-app/internal/domain"
)

type nonIdentifiableStruct struct {
	ID   int64
	Code string
}

func TestBindIdentity(t *testing.T) {
	t.Run("Identifiable domain.Student with int ID", func(t *testing.T) {
		stu := &domain.Student{}
		BindIdentity(stu, "42")
		if stu.ID != 42 || stu.Code != "" {
			t.Fatalf("expected ID=42 Code='', got ID=%d Code=%q", stu.ID, stu.Code)
		}
	})

	t.Run("Identifiable domain.Teacher with string Code", func(t *testing.T) {
		tea := &domain.Teacher{}
		BindIdentity(tea, "TEACHER_MATH")
		if tea.ID != 0 || tea.Code != "TEACHER_MATH" {
			t.Fatalf("expected ID=0 Code='TEACHER_MATH', got ID=%d Code=%q", tea.ID, tea.Code)
		}
	})

	t.Run("Reflection fallback for non-Identifiable struct with int ID", func(t *testing.T) {
		item := &nonIdentifiableStruct{}
		BindIdentity(item, "100")
		if item.ID != 100 || item.Code != "" {
			t.Fatalf("expected ID=100 Code='', got ID=%d Code=%q", item.ID, item.Code)
		}
	})

	t.Run("Reflection fallback for non-Identifiable struct with string Code", func(t *testing.T) {
		item := &nonIdentifiableStruct{}
		BindIdentity(item, "CUSTOM_CODE")
		if item.ID != 0 || item.Code != "CUSTOM_CODE" {
			t.Fatalf("expected ID=0 Code='CUSTOM_CODE', got ID=%d Code=%q", item.ID, item.Code)
		}
	})

	t.Run("Nil pointer safety", func(t *testing.T) {
		var stu *domain.Student
		BindIdentity(stu, "123") // Should not panic
	})
}
