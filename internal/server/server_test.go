package server_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"mygo-app/internal/data"
	"mygo-app/internal/domain"
	"mygo-app/internal/server"
)

func setupTestServer(t *testing.T) (domain.Repo, *httptest.Server) {
	t.Helper()
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "server_test.db")
	repo, err := data.Open(ctx, data.Config{Driver: "sqlite", DSN: dbPath})
	if err != nil {
		t.Fatalf("setup database: %v", err)
	}

	srv := server.New(repo)
	ts := httptest.NewServer(srv)
	t.Cleanup(func() {
		ts.Close()
		_ = repo.Close()
	})
	return repo, ts
}

func TestServerE2E_FullFlow(t *testing.T) {
	_, ts := setupTestServer(t)
	ctx := context.Background()

	// 1. Initialize client without token
	client := data.OpenRemote(ts.URL, "")

	// 2. Auth: Bad password should fail
	_, err := client.Auth().Login(ctx, data.DefaultUsername, "wrongpass")
	if !errors.Is(err, domain.ErrBadCredentials) {
		t.Fatalf("expected ErrBadCredentials, got %v", err)
	}

	// 3. Auth: Login as admin
	user, err := client.Auth().Login(ctx, data.DefaultUsername, data.DefaultPassword)
	if err != nil {
		t.Fatalf("admin login failed: %v", err)
	}
	if user.Username != data.DefaultUsername || user.Role != string(domain.RoleAdmin) {
		t.Fatalf("unexpected user: %+v", user)
	}

	// 4. Stats: Query overview stats
	stats, err := client.Stats(ctx)
	if err != nil {
		t.Fatalf("stats query: %v", err)
	}
	if stats.Students < 0 || stats.Teachers < 0 {
		t.Errorf("invalid stats: %+v", stats)
	}

	// 5. Students CRUD
	newStudent := domain.Student{
		Name:     "张测试",
		Guardian: "张家长",
		Phone:    "13812345678",
		Active:   true,
	}
	if err := client.Students().Save(ctx, &newStudent); err != nil {
		t.Fatalf("create student: %v", err)
	}
	if newStudent.ID == "" {
		t.Fatal("expected generated student ID")
	}

	gotStudent, err := client.Students().Get(ctx, newStudent.ID)
	if err != nil || gotStudent.Name != "张测试" {
		t.Fatalf("get student failed: %v, %+v", err, gotStudent)
	}

	students, err := client.Students().List(ctx, domain.Query{Search: "张测试"})
	if err != nil || len(students) != 1 {
		t.Fatalf("search student failed: %v (count: %d)", err, len(students))
	}

	// 6. Ledger: Top up & Consume
	entry1, err := client.Ledger().TopUp(ctx, newStudent.ID, 100000, "报名预存")
	if err != nil || entry1.Balance != 100000 {
		t.Fatalf("top up failed: %v, %+v", err, entry1)
	}

	entry2, err := client.Ledger().Consume(ctx, newStudent.ID, 20000, "消课一节")
	if err != nil || entry2.Balance != 80000 {
		t.Fatalf("consume failed: %v, %+v", err, entry2)
	}

	entries, err := client.Ledger().List(ctx, newStudent.ID, 10)
	if err != nil || len(entries) != 2 {
		t.Fatalf("list ledger failed: %v (count: %d)", err, len(entries))
	}

	// 7. Teachers CRUD
	teacher := domain.Teacher{Name: "赵老师", Subject: "大提琴", RateCents: 30000, Active: true}
	if err := client.Teachers().Save(ctx, &teacher); err != nil {
		t.Fatalf("save teacher: %v", err)
	}
	tList, err := client.Teachers().List(ctx, domain.Query{Search: "赵老师"})
	if err != nil || len(tList) != 1 {
		t.Fatalf("list teacher failed: %v", err)
	}

	// 8. Classrooms CRUD
	room := domain.Classroom{Name: "琴房 901", Capacity: 6, Location: "九楼"}
	if err := client.Classrooms().Save(ctx, &room); err != nil {
		t.Fatalf("save classroom: %v", err)
	}

	// 9. Courses CRUD
	course := domain.Course{Name: "大提琴高阶班", Subject: "大提琴", TeacherID: teacher.ID, TotalLessons: 12, PriceCents: 200000, Minutes: 45, Active: true}
	if err := client.Courses().Save(ctx, &course); err != nil {
		t.Fatalf("save course: %v", err)
	}

	// 10. Lessons CRUD
	lesson := domain.Lesson{CourseID: course.ID, TeacherID: teacher.ID, ClassroomID: room.ID, Minutes: 45, Status: "planned"}
	if err := client.Lessons().Save(ctx, &lesson); err != nil {
		t.Fatalf("save lesson: %v", err)
	}

	// 11. Settings
	if err := client.Settings().Set(ctx, "theme", "dark"); err != nil {
		t.Fatalf("set settings: %v", err)
	}
	val, err := client.Settings().Get(ctx, "theme")
	if err != nil || val != "dark" {
		t.Fatalf("get settings: %v, want dark", err)
	}

	// 12. Users management
	newUser := domain.User{Username: "viewer1", DisplayName: "观察员", Role: string(domain.RoleViewer)}
	if err := client.Users().Create(ctx, &newUser, "viewer123"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 13. Test RBAC as Viewer: can read, but cannot write
	viewerClient := data.OpenRemote(ts.URL, "")
	_, err = viewerClient.Auth().Login(ctx, "viewer1", "viewer123")
	if err != nil {
		t.Fatalf("viewer login failed: %v", err)
	}

	// Viewer can read students
	if _, err := viewerClient.Students().List(ctx, domain.Query{}); err != nil {
		t.Fatalf("viewer read students failed: %v", err)
	}

	// Viewer write must be blocked by server (403 Forbidden)
	illegalStudent := domain.Student{Name: "非法写入"}
	err = viewerClient.Students().Save(ctx, &illegalStudent)
	if err == nil {
		t.Fatal("viewer write student should have been forbidden by server")
	}
}

func TestServerI18n_AcceptLanguage(t *testing.T) {
	_, ts := setupTestServer(t)

	// 1. Chinese (default): login fail should return Chinese message
	reqZH, _ := http.NewRequest("POST", ts.URL+"/auth/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
	reqZH.Header.Set("Content-Type", "application/json")
	reqZH.Header.Set("Accept-Language", "zh-CN")

	resZH, err := http.DefaultClient.Do(reqZH)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resZH.Body.Close()
	bodyZH, _ := io.ReadAll(resZH.Body)
	if !strings.Contains(string(bodyZH), "用户名或密码不正确") {
		t.Errorf("expected Chinese message '用户名或密码不正确', got: %s", string(bodyZH))
	}

	// 2. English: login fail should return English message
	reqEN, _ := http.NewRequest("POST", ts.URL+"/auth/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
	reqEN.Header.Set("Content-Type", "application/json")
	reqEN.Header.Set("Accept-Language", "en-US")

	resEN, err := http.DefaultClient.Do(reqEN)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resEN.Body.Close()
	bodyEN, _ := io.ReadAll(resEN.Body)
	if !strings.Contains(string(bodyEN), "Invalid username or password") {
		t.Errorf("expected English message 'Invalid username or password', got: %s", string(bodyEN))
	}
}
