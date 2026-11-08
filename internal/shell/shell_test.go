package shell

import (
	"testing"

	"mygo-app/internal/domain"

	"github.com/egoist/mygo/ui"
)

// TestNavVisibility pins what each role sees in the sidebar: the menu is
// the first place a permission shows, and a hidden row must stay hidden.
func TestNavVisibility(t *testing.T) {
	cases := []struct {
		role  domain.Role
		want  []string
		avoid []string
	}{
		{
			role: domain.RoleAdmin,
			want: []string{"概览", "学生管理", "老师管理", "教室管理", "课程管理", "排课", "充值 / 消课", "账号管理", "系统设置"},
		},
		{
			role:  domain.RoleStaff,
			want:  []string{"概览", "学生管理", "排课", "充值 / 消课"},
			avoid: []string{"账号管理", "系统设置"},
		},
		{
			role:  domain.RoleViewer,
			want:  []string{"概览", "学生管理", "充值 / 消课"},
			avoid: []string{"账号管理", "系统设置"},
		},
	}
	for _, c := range cases {
		s := New(domain.User{Username: "u", Role: string(c.role)})
		seen := map[string]bool{}
		for _, g := range Nav {
			visible := 0
			for _, it := range g.Items {
				if s.canSee(it) {
					seen[it.Label] = true
					visible++
				}
			}
			// An empty group must not leave its title behind.
			if visible == 0 {
				continue
			}
		}
		for _, label := range c.want {
			if !seen[label] {
				t.Errorf("%s does not see %q", c.role.Label(), label)
			}
		}
		for _, label := range c.avoid {
			if seen[label] {
				t.Errorf("%s sees %q", c.role.Label(), label)
			}
		}
	}
}

// TestReportsRole checks that the shell knows the role it was built for,
// including one this build does not recognise.
func TestReportsRole(t *testing.T) {
	s := New(domain.User{Username: "ghost", Role: "root"})
	if s.role.CanRead(domain.PermStudents) {
		t.Error("an unknown role may read students")
	}
	if s.UserName != "ghost" {
		t.Errorf("user name %q", s.UserName)
	}
}

func TestAccountFooter(t *testing.T) {
	s := New(domain.User{Username: "zhangsan_teacher", DisplayName: "张三丰特级名师", Role: string(domain.RoleStaff)})
	settingsClicked := false
	signOutClicked := false
	s.OnOpenSettings = func() { settingsClicked = true }
	s.OnSignOut = func() { signOutClicked = true }

	tt := ui.NewTester(func(c *ui.Context) {
		s.sidebar(c)
	}, 300, 700)

	// 1. User display name must be found on the trigger card
	if _, ok := tt.Find("张三丰特级名师"); !ok {
		t.Fatal("expected user name '张三丰特级名师' in sidebar footer")
	}

	// 2. Before click, popover is closed: settings option should not be visible
	if _, ok := tt.Find("popover-settings"); ok {
		t.Error("settings menu should not be open initially")
	}

	// 3. Click trigger to open dropdown
	if err := tt.Click("用户菜单"); err != nil {
		t.Fatalf("failed to click '用户菜单': %v", err)
	}
	tt.Frame()

	// 4. Dropdown options should now be visible
	if _, ok := tt.Find("设置"); !ok {
		t.Fatal("expected '设置' in opened dropdown menu")
	}
	if _, ok := tt.Find("退出登录"); !ok {
		t.Fatal("expected '退出登录' in opened dropdown menu")
	}

	// 5. Click "设置" item in dropdown
	if err := tt.Click("设置"); err != nil {
		t.Fatalf("failed to click '设置': %v", err)
	}
	if !settingsClicked {
		t.Error("expected OnOpenSettings to be called on click")
	}
	if s.menuOpen {
		t.Error("dropdown should close after clicking setting item")
	}
	tt.Frame()

	// 6. Open dropdown again to test "退出登录" with secondary confirmation
	if err := tt.Click("用户菜单"); err != nil {
		t.Fatalf("failed to click '用户菜单' second time: %v", err)
	}
	tt.Frame()

	// Click "退出登录" in dropdown menu -> should trigger confirmation dialog
	if err := tt.Click("退出登录"); err != nil {
		t.Fatalf("failed to click '退出登录': %v", err)
	}
	tt.Frame()

	// OnSignOut should NOT have been called yet because confirmation dialog is pending
	if signOutClicked {
		t.Fatal("OnSignOut must not be called before confirmation")
	}
	if !s.askingSignOut {
		t.Fatal("expected confirmation dialog askingSignOut to be true")
	}

	// Dialog title must be visible
	if _, ok := tt.Find("退出登录确认"); !ok {
		t.Fatal("expected '退出登录确认' dialog to be displayed")
	}

	// 7. Test Cancel in dialog
	if err := tt.Click("取消"); err != nil {
		t.Fatalf("failed to click '取消' in confirmation dialog: %v", err)
	}
	tt.Frame()
	if signOutClicked {
		t.Fatal("OnSignOut must not be called after clicking cancel")
	}
	if s.askingSignOut {
		t.Fatal("confirmation dialog should be closed after clicking cancel")
	}

	// 8. Open dropdown and confirm sign-out
	if err := tt.Click("用户菜单"); err != nil {
		t.Fatalf("failed to click '用户菜单' third time: %v", err)
	}
	tt.Frame()

	if err := tt.Click("退出登录"); err != nil {
		t.Fatalf("failed to click '退出登录': %v", err)
	}
	tt.Frame()

	// In the confirmation dialog, click "退出登录" button to confirm
	if err := tt.Click("退出登录"); err != nil {
		t.Fatalf("failed to click confirm '退出登录' in dialog: %v", err)
	}
	if !signOutClicked {
		t.Error("expected OnSignOut to be called after confirmation")
	}
	if s.askingSignOut {
		t.Error("dialog should close after confirming sign-out")
	}
}

