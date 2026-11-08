package domain

import "testing"

// TestRoleAccess pins the permission matrix: the admin writes everything,
// the staff runs the business but never the system, and the viewer only
// reads.
func TestRoleAccess(t *testing.T) {
	cases := []struct {
		role Role
		perm Permission
		want Access
	}{
		{RoleAdmin, PermStudents, ReadWrite},
		{RoleAdmin, PermSettings, ReadWrite},
		{RoleAdmin, PermUsers, ReadWrite},

		{RoleStaff, PermStudents, ReadWrite},
		{RoleStaff, PermBilling, ReadWrite},
		{RoleStaff, PermLessons, ReadWrite},
		{RoleStaff, PermUsers, NoAccess},
		{RoleStaff, PermSettings, NoAccess},

		{RoleViewer, PermStudents, ReadOnly},
		{RoleViewer, PermBilling, ReadOnly},
		{RoleViewer, PermSettings, NoAccess},
		{RoleViewer, PermUsers, NoAccess},

		// A role this build does not know gets nothing.
		{Role(""), PermStudents, NoAccess},
		{Role("ghost"), PermBilling, NoAccess},
	}
	for _, c := range cases {
		if got := c.role.Access(c.perm); got != c.want {
			t.Errorf("%s.%s = %v, want %v", c.role.Label(), c.perm, got, c.want)
		}
		if c.role.CanRead(c.perm) != (c.want >= ReadOnly) {
			t.Errorf("%s.CanRead(%s) disagrees with Access", c.role.Label(), c.perm)
		}
		if c.role.CanWrite(c.perm) != (c.want >= ReadWrite) {
			t.Errorf("%s.CanWrite(%s) disagrees with Access", c.role.Label(), c.perm)
		}
	}
}

// TestParseRole checks that a stored value round trips, and that anything
// else falls back to no access rather than to the admin.
func TestParseRole(t *testing.T) {
	for _, r := range Roles {
		if got := ParseRole(string(r)); got != r {
			t.Errorf("ParseRole(%q) = %q", r, got)
		}
	}
	for _, s := range []string{"", "root", "ADMIN", "超级管理员"} {
		if got := ParseRole(s); got.CanRead(PermStudents) {
			t.Errorf("ParseRole(%q) = %q, which may read", s, got)
		}
	}
}
