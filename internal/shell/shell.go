// Package shell is the frame of the app: the navigation on the left, the
// router on the right. It is the only place that knows which screens
// exist, so adding a page is one entry in Nav and one case in the switch.
package shell

import (
	"runtime"
	"time"

	"mygo-app/internal/design"
	"mygo-app/internal/design/tokens"
	"mygo-app/internal/domain"
	"mygo-app/internal/features"
	"mygo-app/internal/i18n"

	"github.com/egoist/mygo/ui"
)

// page is what the shell asks of a screen.
type page interface {
	View(c *ui.Context)
}

// NavItem is one entry of the sidebar. Its path is both the route the
// router shows and the value the row is chosen by. Perm is the part of the
// app it needs; the row appears only for a role that may read it.
type NavItem struct {
	Path  string
	Label string
	Perm  domain.Permission
}

// NavGroup is a titled group of entries, as 业务 and 系统.
type NavGroup struct {
	Title string
	Items []NavItem
}

// Nav is the single source of truth of the app's sections: the sidebar
// draws it and the router switches on the same paths.
var Nav = []NavGroup{
	{"业务", []NavItem{
		{"/dashboard", "概览", ""},
		{"/students", "学生管理", domain.PermStudents},
		{"/teachers", "老师管理", domain.PermTeachers},
		{"/classrooms", "教室管理", domain.PermClassrooms},
		{"/courses", "课程管理", domain.PermCourses},
		{"/lessons", "排课", domain.PermLessons},
		{"/billing", "充值 / 消课", domain.PermBilling},
	}},
	{"系统", []NavItem{
		{"/users", "账号管理", domain.PermUsers},
		{"/settings", "系统设置", domain.PermSettings},
	}},
}

// Shell holds the router and one state object per screen. The screens keep
// their own state, so switching pages does not reload what is already
// loaded.
type Shell struct {
	repo   domain.Repo
	win    design.Updater
	Router *ui.Router
	nav    string
	self   domain.User
	role   domain.Role

	// UserName is who signed in, shown in the sidebar's footer.
	UserName string
	// OnSignOut runs when the footer's button is pressed.
	OnSignOut func()
	// OnOpenSettings runs when the settings button is pressed.
	OnOpenSettings func()

	menuOpen      bool // controls bottom user dropdown menu
	askingSignOut bool // controls sign-out confirmation dialog

	dashboard  page
	students   page
	teachers   page
	classrooms page
	courses    page
	lessons    page
	billing    page
	users      page
	settings   page
}

// New builds the shell before the window exists; Attach connects it once
// the window is, because the screens reload through Window.Update. The
// role of the signed-in user decides what the shell offers at all.
func New(self domain.User) *Shell {
	return &Shell{
		Router:   ui.NewRouter("/dashboard"),
		nav:      "/dashboard",
		self:     self,
		role:     domain.ParseRole(self.Role),
		UserName: self.Name(),
	}
}

// Attach connects the store and the window, and builds the screens. Each
// screen learns whether the role may write in it, so that a read-only user
// never sees a button that would fail.
func (s *Shell) Attach(repo domain.Repo, win design.Updater) {
	s.repo, s.win = repo, win
	s.dashboard = features.NewDashboard(repo, win)
	s.students = features.NewStudents(repo, win, s.role.CanWrite(domain.PermStudents))
	s.teachers = features.NewTeachers(repo, win, s.role.CanWrite(domain.PermTeachers))
	s.classrooms = features.NewClassrooms(repo, win, s.role.CanWrite(domain.PermClassrooms))
	s.courses = features.NewCourses(repo, win, s.role.CanWrite(domain.PermCourses))
	s.lessons = features.NewLessons(repo, win, s.role.CanWrite(domain.PermLessons))
	s.billing = features.NewBilling(repo, win, s.role.CanWrite(domain.PermBilling))
	s.users = features.NewUsers(repo, win, s.self, func(u domain.User) {
		s.self, s.UserName = u, u.Name()
	})
	s.settings = features.NewSettings(repo, win, s.self)
}

// View builds the window: the sidebar, then the page the router shows.
func (s *Shell) View(c *ui.Context) {
	if s.repo == nil {
		design.Page(c, design.PageProps{Title: "正在启动…"}, func() {
			design.Loading(c, "数据")
		})
		return
	}
	look := design.LookFor(c)
	if p := s.Router.Path(); p != s.nav {
		s.nav = p
	}

	ui.Row(c).Fill().Background(look.Page).AlignItems(ui.Stretch).Children(func() {
		s.sidebar(c)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			s.Router.View(c, func(r *ui.Route) {
				// A new path is a new element, so the page fades and
				// rises into place instead of appearing.
				page := ui.Column(c).Key(s.Router.Path()).Fill().Grow(1).MinHeight(0).
					Transition(ui.ElementTransition{
						Duration: 220 * time.Millisecond,
						Enter:    &ui.Motion{Y: 10, Opacity: 0},
					})
				page.Children(func() {
					switch {
					case r.Match("/dashboard"):
						r.Title(navTitle("/dashboard", "概览"))
						s.dashboard.View(c)
					case r.Match("/students"):
						r.Title(navTitle("/students", "学生管理"))
						s.serve(c, domain.PermStudents, s.students)
					case r.Match("/teachers"):
						r.Title(navTitle("/teachers", "老师管理"))
						s.serve(c, domain.PermTeachers, s.teachers)
					case r.Match("/classrooms"):
						r.Title(navTitle("/classrooms", "教室管理"))
						s.serve(c, domain.PermClassrooms, s.classrooms)
					case r.Match("/courses"):
						r.Title(navTitle("/courses", "课程管理"))
						s.serve(c, domain.PermCourses, s.courses)
					case r.Match("/lessons"):
						r.Title(navTitle("/lessons", "排课"))
						s.serve(c, domain.PermLessons, s.lessons)
					case r.Match("/billing"):
						r.Title(navTitle("/billing", "充值 / 消课"))
						s.serve(c, domain.PermBilling, s.billing)
					case r.Match("/users"):
						r.Title(navTitle("/users", "账号管理"))
						s.serve(c, domain.PermUsers, s.users)
					case r.Match("/settings"):
						r.Title(navTitle("/settings", "系统设置"))
						s.serve(c, domain.PermSettings, s.settings)
					default:
						design.Empty(c, "页面不存在", s.Router.Path())
					}
				})
			})
		})
	})
}

func navTitle(path, fallback string) string {
	switch path {
	case "/dashboard":
		return i18n.T("nav.overview")
	case "/students":
		return i18n.T("nav.students")
	case "/teachers":
		return i18n.T("nav.teachers")
	case "/classrooms":
		return i18n.T("nav.classrooms")
	case "/courses":
		return i18n.T("nav.courses")
	case "/lessons":
		return i18n.T("nav.lessons")
	case "/billing":
		return i18n.T("nav.ledger")
	case "/users":
		return i18n.T("nav.users")
	case "/settings":
		return i18n.T("nav.settings")
	default:
		return fallback
	}
}

// sidebar draws the brand, the navigation and the account at the bottom.
func (s *Shell) sidebar(c *ui.Context) {
	t := design.Theme(c)
	next := ""
	ui.Column(c).Width(design.SideWidth).Shrink(0).Padding(14, 10).Gap(2).
		Background(design.LookFor(c).Card).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
		ui.Row(c).Padding(0, 6, 18).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "启明培训").FontSize(15).Bold()
			ui.Spacer(c)
			design.Pill(c, "后台", design.ToneAccent)
		})
		for _, group := range Nav {
			visible := make([]NavItem, 0, len(group.Items))
			for _, it := range group.Items {
				if s.canSee(it) {
					visible = append(visible, it)
				}
			}
			if len(visible) == 0 {
				continue
			}
			ui.Text(c, group.Title).FontSize(11).TextColor(t.TextMuted).Padding(10, 10, 4)
			for _, it := range visible {
				if design.NavRow(c, navTitle(it.Path, it.Label), it.Path == s.nav) {
					next = it.Path
				}
			}
		}
		ui.Spacer(c)
		s.account(c)
	})
	if next != "" && next != s.nav {
		s.nav = next
		s.Router.Push(next)
	}
}

// account is the signed-in user and the dropdown menu.
func (s *Shell) account(c *ui.Context) {
	t := design.Theme(c)
	tok := tokens.Get(c)

	ui.Column(c).Padding(6, 0, 0).
		BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {

		// Trigger card: user identity + dropdown caret
		trigger := ui.Row(c.Key("user-dropdown-trigger")).
			Label("用户菜单").
			Padding(6, 8).Radius(tokens.RadiusMd).Gap(8).AlignItems(ui.Center).
			Cursor(ui.CursorPointer)

		if trigger.Hovered() || s.menuOpen {
			trigger.Background(t.SurfacePressed)
		}

		trigger.Children(func() {
			// Avatar circle
			ui.Box(c).Size(28, 28).Radius(14).Background(t.SurfacePressed).Center().Children(func() {
				ui.Text(c, initial(s.UserName)).FontSize(12).Bold().TextColor(t.Text)
			})
			// Username + Role (full available width, never squeezed)
			ui.Column(c).Grow(1).Gap(1).MinWidth(0).Children(func() {
				ui.Text(c, s.UserName).FontSize(13).Bold().SingleLine()
				ui.Text(c, s.role.Label()).FontSize(10).TextColor(t.TextMuted)
			})
			// Dropdown caret indicator
			caret := "▾"
			if s.menuOpen {
				caret = "▴"
			}
			ui.Text(c, caret).FontSize(11).TextColor(t.TextMuted)
		})

		if trigger.Clicked() {
			s.menuOpen = !s.menuOpen
		}

		// Dropdown menu popover
		ui.Popover(c, trigger, &s.menuOpen, func() {
			ui.Column(c).Width(204).Gap(2).Padding(4).Children(func() {
				// 1. Profile summary header inside dropdown
				ui.Row(c).Padding(8, 10).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(32, 32).Radius(16).Background(t.SurfacePressed).Center().Children(func() {
						ui.Text(c, initial(s.UserName)).FontSize(13).Bold().TextColor(t.Text)
					})
					ui.Column(c).Grow(1).Gap(2).MinWidth(0).Children(func() {
						ui.Text(c, s.UserName).FontSize(13).Bold().SingleLine()
						ui.Text(c, s.role.Label()).FontSize(11).TextColor(t.TextMuted)
					})
				})

				// Divider
				ui.Row(c).Height(1).Background(t.Border).Margin(2, 0)

				// 2. Settings action
				if s.OnOpenSettings != nil {
					settingsItem := ui.Row(c.Key("popover-settings")).
						Label(i18n.T("settings.title")).
						Padding(7, 10).Radius(tokens.RadiusSm).Gap(8).AlignItems(ui.Center).
						Cursor(ui.CursorPointer)
					if settingsItem.Hovered() {
						settingsItem.Background(t.SurfacePressed)
					}
					settingsItem.Children(func() {
						ui.Text(c, "⚙").FontSize(13)
						ui.Text(c, i18n.T("settings.title")).FontSize(12)
						ui.Spacer(c)
						shortcut := "⌘,"
						if runtime.GOOS != "darwin" {
							shortcut = "Ctrl+,"
						}
						ui.Text(c, shortcut).FontSize(11).TextColor(t.TextMuted)
					})
					if settingsItem.Clicked() {
						s.menuOpen = false
						s.OnOpenSettings()
					}
				}

				// Divider
				ui.Row(c).Height(1).Background(t.Border).Margin(2, 0)

				// 3. Sign out action
				if s.OnSignOut != nil {
					signOutItem := ui.Row(c.Key("popover-signout")).
						Label(i18n.T("btn.signout")).
						Padding(7, 10).Radius(tokens.RadiusSm).Gap(8).AlignItems(ui.Center).
						Cursor(ui.CursorPointer)
					if signOutItem.Hovered() {
						signOutItem.Background(tok.Danger.Alpha(0.12))
					}
					signOutItem.Children(func() {
						ui.Text(c, "🚪").FontSize(13)
						lbl := ui.Text(c, i18n.T("btn.signout")).FontSize(12)
						if signOutItem.Hovered() {
							lbl.TextColor(tok.Danger)
						}
					})
					if signOutItem.Clicked() {
						s.menuOpen = false
						s.askingSignOut = true
					}
				}
			})
		})

		if ui.AlertDialog(c, &s.askingSignOut,
			i18n.T("auth.signout_title"),
			i18n.T("auth.signout_desc"),
			i18n.T("btn.cancel"),
			i18n.T("btn.signout")) == 1 {
			if s.OnSignOut != nil {
				s.OnSignOut()
			}
		}
	})
}

// initial is the first rune of the name, for the avatar.
func initial(name string) string {
	for _, r := range name {
		return string(r)
	}
	return "?"
}

// canSee reports whether the role may open a part of the app: a row
// without a permission, as 概览, is for everyone signed in.
func (s *Shell) canSee(it NavItem) bool {
	return it.Perm == "" || s.role.CanRead(it.Perm)
}

// serve builds a screen only when the role may read it, so that a deep
// link gets the same answer as a row the sidebar never showed.
func (s *Shell) serve(c *ui.Context, perm domain.Permission, p page) {
	if !s.role.CanRead(perm) {
		design.Page(c, design.PageProps{Title: "没有权限"}, func() {
			design.Empty(c, "当前角色不能访问这一页",
				s.role.Label()+"没有这个模块的权限，请联系管理员调整。")
		})
		return
	}
	p.View(c)
}
