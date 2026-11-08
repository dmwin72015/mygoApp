package features_test

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"mygo-app/internal/captcha"
	"mygo-app/internal/data"
	"mygo-app/internal/domain"
	"mygo-app/internal/features"

	"github.com/egoist/mygo/ui"
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

func TestLoginForgotPasswordUI(t *testing.T) {
	repo := openTestRepo(t)
	u := newTestUpdater()
	var signedInUser domain.User
	login := features.NewLogin(repo, u, func(u domain.User) {
		signedInUser = u
	})

	tt := ui.NewTester(func(c *ui.Context) {
		login.View(c)
	}, 600, 500)

	// Initially on Sign In screen
	if _, ok := tt.Find("忘记密码？"); !ok {
		t.Fatal("expected '忘记密码？' link on login screen")
	}

	// Click "忘记密码？"
	if err := tt.Click("忘记密码？"); err != nil {
		t.Fatalf("failed to click '忘记密码？': %v", err)
	}
	tt.Frame()

	// Should now be on Forgot Password screen
	if _, ok := tt.Find("确认重置密码"); !ok {
		t.Fatal("expected '确认重置密码' button on forgot password screen")
	}
	if _, ok := tt.Find("返回登录"); !ok {
		t.Fatal("expected '返回登录' button on forgot password screen")
	}

	// Click "返回登录"
	if err := tt.Click("返回登录"); err != nil {
		t.Fatalf("failed to click '返回登录': %v", err)
	}
	tt.Frame()

	// Should be back to Sign In screen
	if _, ok := tt.Find("登录"); !ok {
		t.Fatal("expected '登录' button after returning to login screen")
	}

	// Now trigger password reset flow
	login.ResetPasswordForTest(data.DefaultUsername, domain.DefaultRecoveryKey, "newPassFromForgot666", "newPassFromForgot666")
	u.wait(t)
	tt.Frame()

	// Verify old password fails
	if _, err := repo.Auth().Login(context.Background(), data.DefaultUsername, data.DefaultPassword); err == nil {
		t.Fatal("old password should fail after reset")
	}

	// Verify new password succeeds
	if _, err := repo.Auth().Login(context.Background(), data.DefaultUsername, "newPassFromForgot666"); err != nil {
		t.Fatalf("new password should succeed after reset: %v", err)
	}

	_ = signedInUser
}

func TestLoginSignUpUI(t *testing.T) {
	repo := openTestRepo(t)
	u := newTestUpdater()
	var signedInUser domain.User
	login := features.NewLogin(repo, u, func(user domain.User) {
		signedInUser = user
	})

	tt := ui.NewTester(func(c *ui.Context) {
		login.View(c)
	}, 600, 500)

	// 1. Initially on Sign In screen, verify registration link exists
	signUpLink := "还没有账号？立即注册"
	if _, ok := tt.Find(signUpLink); !ok {
		t.Fatalf("expected '%s' on login screen", signUpLink)
	}

	// 2. Click "还没有账号？立即注册"
	if err := tt.Click(signUpLink); err != nil {
		t.Fatalf("failed to click '%s': %v", signUpLink, err)
	}
	tt.Frame()

	// 3. Should now be on Sign Up card
	if _, ok := tt.Find("立即注册"); !ok {
		t.Fatal("expected '立即注册' button on sign up screen")
	}
	if _, ok := tt.Find("已有账号？返回登录"); !ok {
		t.Fatal("expected '已有账号？返回登录' button on sign up screen")
	}

	// 4. Click "已有账号？返回登录"
	if err := tt.Click("已有账号？返回登录"); err != nil {
		t.Fatalf("failed to click '已有账号？返回登录': %v", err)
	}
	tt.Frame()

	// Should be back to Sign In screen
	if _, ok := tt.Find("登录"); !ok {
		t.Fatal("expected '登录' button after returning to login screen")
	}

	// 5. Test validation in registration: mismatch passwords
	login.SignUpForTest("new_teacher", "pass123456", "different_pass", "Teacher Wang")
	tt.Frame()
	if _, ok := tt.Find("两次输入的密码不一致"); !ok {
		t.Fatal("expected password mismatch error")
	}

	// 6. Successful registration
	login.SignUpForTest("new_teacher", "pass123456", "pass123456", "王老师")
	u.wait(t)
	tt.Frame()

	if signedInUser.Username != "new_teacher" {
		t.Fatalf("expected signed in user 'new_teacher', got: %s", signedInUser.Username)
	}
	if signedInUser.DisplayName != "王老师" {
		t.Fatalf("expected display name '王老师', got: %s", signedInUser.DisplayName)
	}
	if signedInUser.Role != string(domain.RoleStaff) {
		t.Fatalf("expected role staff, got: %s", signedInUser.Role)
	}

	// 7. Verify login directly with the newly registered user
	u2, err := repo.Auth().Login(context.Background(), "new_teacher", "pass123456")
	if err != nil {
		t.Fatalf("login with registered user failed: %v", err)
	}
	if u2.Username != "new_teacher" {
		t.Fatalf("unexpected user: %+v", u2)
	}
}

func TestLoginSettingsButtonPlatform(t *testing.T) {
	repo := openTestRepo(t)
	u := newTestUpdater()
	login := features.NewLogin(repo, u, func(user domain.User) {})
	login.OnOpenSettings = func() {}

	tt := ui.NewTester(func(c *ui.Context) {
		login.View(c)
	}, 600, 500)

	btnTitle := "⚙ " + "设置"
	_, found := tt.Find(btnTitle)
	// On darwin (macOS), the settings button should NOT appear on the login screen.
	// On other platforms (windows, linux), it should appear.
	if runtime.GOOS == "darwin" && found {
		t.Errorf("expected settings button to be hidden on darwin login screen, but found it")
	}
}

type mockFeatureCaptchaService struct {
	expectedAnswer string
}

func (m *mockFeatureCaptchaService) Generate() (captcha.Item, error) {
	return captcha.Item{
		ID:    "mock-id-1",
		Image: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
	}, nil
}

func (m *mockFeatureCaptchaService) Verify(id, answer string) bool {
	return id == "mock-id-1" && answer == m.expectedAnswer
}

func TestLoginCaptchaFlow(t *testing.T) {
	repo := openTestRepo(t)
	u := newTestUpdater()
	var signedInUser domain.User

	login := features.NewLogin(repo, u, func(user domain.User) {
		signedInUser = user
	})
	mockSvc := &mockFeatureCaptchaService{expectedAnswer: "77"}
	login.SetLocalCaptchaService(mockSvc)

	// 1. Initial render
	tt := ui.NewTester(func(c *ui.Context) {
		login.View(c)
	}, 600, 500)

	if _, ok := tt.Find("验证码"); !ok {
		t.Fatal("expected '验证码' field on login card")
	}

	// 2. Submit with empty captcha
	login.SignInForTest(data.DefaultUsername, data.DefaultPassword, "")
	if !strings.Contains(login.FormError(), "验证码") {
		t.Fatalf("expected captcha required error, got: %q", login.FormError())
	}

	// 3. Submit with wrong captcha
	login.SignInForTest(data.DefaultUsername, data.DefaultPassword, "999")
	if !strings.Contains(login.FormError(), "验证码错误") {
		t.Fatalf("expected bad captcha error, got: %q", login.FormError())
	}

	// 4. Submit with correct captcha answer
	login.SignInForTest(data.DefaultUsername, data.DefaultPassword, "77")
	u.wait(t)

	if signedInUser.Username != data.DefaultUsername {
		t.Fatalf("expected signed in user %q, got: %q (err: %s)", data.DefaultUsername, signedInUser.Username, login.FormError())
	}
}



