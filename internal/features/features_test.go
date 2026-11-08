package features_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"mygo-app/internal/data"
	"mygo-app/internal/domain"
	"mygo-app/internal/features"
)

type testUpdater struct {
	updated chan struct{}
}

func newTestUpdater() *testUpdater {
	return &testUpdater{updated: make(chan struct{}, 100)}
}

func (u *testUpdater) Update(fn func()) {
	if fn != nil {
		fn()
	}
	u.updated <- struct{}{}
}

func (u *testUpdater) wait(t *testing.T) {
	t.Helper()
	select {
	case <-u.updated:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for feature update")
	}
}

func openTestRepo(t *testing.T) domain.Repo {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_school.db")
	repo, err := data.Open(ctx, data.Config{Driver: "sqlite", DSN: dbPath})
	if err != nil {
		t.Fatalf("data.Open failed: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

// TestTeachersCRUD verifies the Teacher feature drawer lifecycle headless.
func TestTeachersCRUD(t *testing.T) {
	repo := openTestRepo(t)
	u := newTestUpdater()
	f := features.NewTeachers(repo, u, true)

	// 1. Initial reload
	f.ReloadForTest()
	u.wait(t)

	// 2. Create new teacher
	f.NewTeacherForTest("赵特级教师", "大提琴", "13800138999", 35000)
	u.wait(t) // save completes
	u.wait(t) // reload completes

	// 3. Verify
	teachers, err := repo.Teachers().List(context.Background(), domain.Query{Search: "赵特级教师"})
	if err != nil || len(teachers) != 1 {
		t.Fatalf("expected 1 teacher matching name, got %d (err: %v)", len(teachers), err)
	}
	if teachers[0].Name != "赵特级教师" || teachers[0].RateCents != 35000 {
		t.Errorf("teacher data mismatch: %+v", teachers[0])
	}
}

// TestClassroomsCRUD verifies Classroom management.
func TestClassroomsCRUD(t *testing.T) {
	repo := openTestRepo(t)
	u := newTestUpdater()
	f := features.NewClassrooms(repo, u, true)

	f.ReloadForTest()
	u.wait(t)

	f.NewClassroomForTest("琴房 888", 6, "A座 801")
	u.wait(t) // save
	u.wait(t) // reload

	rooms, err := repo.Classrooms().List(context.Background(), domain.Query{Search: "888"})
	if err != nil || len(rooms) != 1 {
		t.Fatalf("expected 1 classroom matching 888, got %d (err: %v)", len(rooms), err)
	}
	if rooms[0].Name != "琴房 888" || rooms[0].Capacity != 6 {
		t.Errorf("classroom data mismatch: %+v", rooms[0])
	}
}

// TestCoursesCRUD verifies Course management.
func TestCoursesCRUD(t *testing.T) {
	repo := openTestRepo(t)
	u := newTestUpdater()
	f := features.NewCourses(repo, u, true)

	f.ReloadForTest()
	u.wait(t)

	f.NewCourseForTest("大提琴精品班", "器乐", 16, 60, 240000)
	u.wait(t) // save
	u.wait(t) // reload

	courses, err := repo.Courses().List(context.Background(), domain.Query{Search: "大提琴精品班"})
	if err != nil || len(courses) != 1 {
		t.Fatalf("expected 1 course matching name, got %d (err: %v)", len(courses), err)
	}
	if courses[0].Name != "大提琴精品班" || courses[0].PriceCents != 240000 {
		t.Errorf("course data mismatch: %+v", courses[0])
	}
}

// TestLessonsLifecycle verifies Lesson management and status transition.
func TestLessonsLifecycle(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()

	// Seed course, teacher, room
	course := domain.Course{Name: "小提琴课", PriceCents: 10000, TotalLessons: 1}
	_ = repo.Courses().Save(ctx, &course)
	teacher := domain.Teacher{Name: "林老师", Active: true}
	_ = repo.Teachers().Save(ctx, &teacher)
	room := domain.Classroom{Name: "教室 1", Capacity: 10}
	_ = repo.Classrooms().Save(ctx, &room)

	u := newTestUpdater()
	f := features.NewLessons(repo, u, true)
	f.ReloadForTest()
	u.wait(t)

	// Create lesson
	f.NewLessonForTest("小提琴课", "林老师", "教室 1", "2026-11-10 14:00", 60, "待上课")
	u.wait(t) // save
	u.wait(t) // reload

	lessons, err := repo.Lessons().List(ctx, domain.Query{})
	if err != nil {
		t.Fatalf("list lessons err: %v", err)
	}
	found := false
	for _, l := range lessons {
		if l.Minutes == 60 && l.Status == "planned" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("created lesson not found in %+v", lessons)
	}
}

// TestSettingsChangePassword verifies user password change in Settings.
func TestSettingsChangePassword(t *testing.T) {
	repo := openTestRepo(t)
	ctx := context.Background()
	user, err := repo.Auth().Login(ctx, data.DefaultUsername, data.DefaultPassword)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	u := newTestUpdater()
	s := features.NewSettings(repo, u, user)

	// Change password
	s.ChangePasswordForTest("admin123", "newAdmin888", "newAdmin888")
	u.wait(t) // login verify + set password update

	// Verify old password fails and new password succeeds
	if _, err := repo.Auth().Login(ctx, data.DefaultUsername, "admin123"); err == nil {
		t.Error("old password should fail")
	}
	if _, err := repo.Auth().Login(ctx, data.DefaultUsername, "newAdmin888"); err != nil {
		t.Errorf("new password should succeed: %v", err)
	}
}
