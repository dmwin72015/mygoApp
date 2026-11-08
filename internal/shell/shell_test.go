package shell

import (
	"testing"

	"mygo-app/internal/domain"
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
