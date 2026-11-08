package server_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
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
	if newStudent.ID == 0 {
		t.Fatal("expected generated student ID")
	}

	gotStudent, err := client.Students().Get(ctx, strconv.FormatInt(newStudent.ID, 10))
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

func TestServerResetPassword(t *testing.T) {
	_, ts := setupTestServer(t)
	ctx := context.Background()
	client := data.OpenRemote(ts.URL, "")

	// 1. Wrong recovery key
	err := client.Auth().ResetPassword(ctx, data.DefaultUsername, "bad-key", "newPassword123")
	if !errors.Is(err, domain.ErrInvalidRecoveryKey) {
		t.Fatalf("expected ErrInvalidRecoveryKey, got %v", err)
	}

	// 2. Non-existent user
	err = client.Auth().ResetPassword(ctx, "ghost_user", domain.DefaultRecoveryKey, "newPassword123")
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}

	// 3. Successful reset
	err = client.Auth().ResetPassword(ctx, data.DefaultUsername, domain.DefaultRecoveryKey, "adminNewPass666")
	if err != nil {
		t.Fatalf("expected successful reset, got %v", err)
	}

	// 4. Verify login with new password
	u, err := client.Auth().Login(ctx, data.DefaultUsername, "adminNewPass666")
	if err != nil {
		t.Fatalf("login with new password failed: %v", err)
	}
	if u.Username != data.DefaultUsername {
		t.Fatalf("unexpected user: %+v", u)
	}
}

func TestServerAuth_Register(t *testing.T) {
	_, ts := setupTestServer(t)
	ctx := context.Background()
	client := data.OpenRemote(ts.URL, "")

	// 1. Successful registration via client
	u, err := client.Auth().Register(ctx, "teacher_anna", "password123", "Anna Teacher")
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}
	if u.Username != "teacher_anna" || u.DisplayName != "Anna Teacher" || u.Role != string(domain.RoleStaff) {
		t.Fatalf("unexpected registered user: %+v", u)
	}

	// 2. Client token was populated by Register, so authenticated requests work
	_, err = client.Students().List(ctx, domain.Query{})
	if err != nil {
		t.Fatalf("request with auto-authenticated token failed: %v", err)
	}

	// 3. Duplicate registration via client should fail
	_, err = client.Auth().Register(ctx, "teacher_anna", "password456", "Anna Another")
	if err == nil {
		t.Fatalf("expected error on duplicate registration, got nil")
	}

	// 4. Validation errors via raw HTTP requests
	// 4a. Short password
	shortBody := `{"username":"bob","password":"123","display_name":"Bob"}`
	res, err := http.Post(ts.URL+"/auth/register", "application/json", strings.NewReader(shortBody))
	if err != nil {
		t.Fatalf("http request failed: %v", err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for short password, got %d", res.StatusCode)
	}
	res.Body.Close()

	// 4b. Empty username
	emptyUserBody := `{"username":"","password":"validpassword123","display_name":"Empty"}`
	res, err = http.Post(ts.URL+"/auth/register", "application/json", strings.NewReader(emptyUserBody))
	if err != nil {
		t.Fatalf("http request failed: %v", err)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for empty username, got %d", res.StatusCode)
	}
	res.Body.Close()

	// 4c. Duplicate username returns 409 Conflict
	dupBody := `{"username":"teacher_anna","password":"validpassword123","display_name":"Anna"}`
	res, err = http.Post(ts.URL+"/auth/register", "application/json", strings.NewReader(dupBody))
	if err != nil {
		t.Fatalf("http request failed: %v", err)
	}
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate username, got %d", res.StatusCode)
	}
	res.Body.Close()
}

func TestServerHealthAndPing(t *testing.T) {
	_, ts := setupTestServer(t)

	// 1. Test /ping
	res, err := http.Get(ts.URL + "/ping")
	if err != nil {
		t.Fatalf("GET /ping failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /ping, got %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "pong") {
		t.Fatalf("expected 'pong' in response, got: %s", string(body))
	}

	// 2. Test /health
	resH, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resH.Body.Close()
	if resH.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /health, got %d", resH.StatusCode)
	}
	bodyH, _ := io.ReadAll(resH.Body)
	if !strings.Contains(string(bodyH), `"status":"ok"`) || !strings.Contains(string(bodyH), `"db":"up"`) {
		t.Fatalf("expected status:ok and db:up in health response, got: %s", string(bodyH))
	}
}

func TestServerRequestID(t *testing.T) {
	_, ts := setupTestServer(t)

	// 1. Without incoming X-Request-ID (should auto-generate req_...)
	res, err := http.Get(ts.URL + "/ping")
	if err != nil {
		t.Fatalf("GET /ping failed: %v", err)
	}
	res.Body.Close()
	reqID := res.Header.Get("X-Request-ID")
	if !strings.HasPrefix(reqID, "req_") {
		t.Fatalf("expected auto-generated X-Request-ID starting with 'req_', got: %s", reqID)
	}

	// 2. With client-provided X-Request-ID (should propagate)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/ping", nil)
	req.Header.Set("X-Request-ID", "client-trace-12345")
	res2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /ping with custom ID failed: %v", err)
	}
	res2.Body.Close()
	if res2.Header.Get("X-Request-ID") != "client-trace-12345" {
		t.Fatalf("expected propagated X-Request-ID 'client-trace-12345', got: %s", res2.Header.Get("X-Request-ID"))
	}
}



