package domain

// Role is what a user may do. It is stored as text, so that a new role
// needs no migration, and an unknown one gets nothing.
type Role string

// The roles the app ships with.
const (
	RoleAdmin  Role = "admin"  // 管理员：全部
	RoleStaff  Role = "staff"  // 教务：日常业务，不能改系统与人员
	RoleViewer Role = "viewer" // 只读：只能看
)

// Roles lists them in the order the UI shows them.
var Roles = []Role{RoleAdmin, RoleStaff, RoleViewer}

// ParseRole reads what was stored, defaulting to no access at all.
func ParseRole(s string) Role {
	switch Role(s) {
	case RoleAdmin:
		return RoleAdmin
	case RoleStaff:
		return RoleStaff
	case RoleViewer:
		return RoleViewer
	}
	return Role("")
}

// Label is what the UI calls the role.
func (r Role) Label() string {
	switch r {
	case RoleAdmin:
		return "管理员"
	case RoleStaff:
		return "教务"
	case RoleViewer:
		return "只读"
	}
	return "未知角色"
}

// Permission names one part of the app.
type Permission string

// The parts the app guards.
const (
	PermStudents   Permission = "students"
	PermTeachers   Permission = "teachers"
	PermClassrooms Permission = "classrooms"
	PermCourses    Permission = "courses"
	PermLessons    Permission = "lessons"
	PermBilling    Permission = "billing"
	PermSettings   Permission = "settings"
	PermUsers      Permission = "users"
)

// Access is how far a role goes in one part of the app.
type Access int

// The levels of access, from none to writing.
const (
	NoAccess Access = iota
	ReadOnly
	ReadWrite
)

// CanRead reports whether the role may open the part at all.
func (r Role) CanRead(p Permission) bool { return r.Access(p) >= ReadOnly }

// CanWrite reports whether the role may change what the part holds.
func (r Role) CanWrite(p Permission) bool { return r.Access(p) >= ReadWrite }

// Access returns what the role may do in a part. The matrix lives here,
// in one table, so that a question about permissions has one answer:
// the admin writes everything, an unknown role reads nothing.
func (r Role) Access(p Permission) Access {
	if r == RoleAdmin {
		return ReadWrite
	}
	if lvl, ok := matrix[r]; ok {
		return lvl[p]
	}
	return NoAccess
}

// matrix is the roles below the admin. A part left out is one the role
// cannot open.
var matrix = map[Role]map[Permission]Access{
	RoleStaff: {
		PermStudents:   ReadWrite,
		PermTeachers:   ReadWrite,
		PermClassrooms: ReadWrite,
		PermCourses:    ReadWrite,
		PermLessons:    ReadWrite,
		PermBilling:    ReadWrite,
		// Settings and users belong to the admin.
	},
	RoleViewer: {
		PermStudents:   ReadOnly,
		PermTeachers:   ReadOnly,
		PermClassrooms: ReadOnly,
		PermCourses:    ReadOnly,
		PermLessons:    ReadOnly,
		PermBilling:    ReadOnly,
	},
}
