package features

import (
	"context"
	"fmt"
	"strings"

	"mygo-app/internal/design"
	"mygo-app/internal/domain"

	"github.com/egoist/mygo/ui"
)

// Users is 账号管理: who may sign in, and with which role. Only an admin
// reaches it, which the shell enforces before building this page.
type Users struct {
	repo domain.Repo
	win  design.Updater
	self domain.User
	// onSelf runs when the admin changes their own account, so that the
	// sidebar shows the new name without a restart.
	onSelf func(domain.User)

	rows         []domain.User
	loaded, busy bool
	err          error

	tbl ui.ListState
	sel int

	editing  bool
	isNew    bool
	draft    domain.User
	password string
	formErr  string

	askingDelete bool
}

// NewUsers builds the page. self is the signed-in admin, who may not
// delete themself.
func NewUsers(repo domain.Repo, win design.Updater, self domain.User, onSelf func(domain.User)) *Users {
	return &Users{repo: repo, win: win, self: self, onSelf: onSelf, sel: -1}
}

func (u *Users) reload() {
	u.busy = true
	win, repo := u.win, u.repo
	go func() {
		rows, err := repo.Users().List(context.Background())
		win.Update(func() {
			u.rows, u.err, u.busy, u.loaded = rows, err, false, true
			if u.sel >= len(u.rows) {
				u.sel = len(u.rows) - 1
			}
			if u.sel < -1 {
				u.sel = -1
			}
		})
	}()
}

func (u *Users) startNew() {
	u.draft = domain.User{Role: string(domain.RoleStaff)}
	u.password, u.isNew, u.editing, u.formErr = "", true, true, ""
}

func (u *Users) edit(v domain.User) {
	u.draft = v
	u.password, u.isNew, u.editing, u.formErr = "", false, true, ""
}

func (u *Users) save() {
	u.draft.Username = strings.TrimSpace(u.draft.Username)
	if u.draft.Username == "" {
		u.formErr = "用户名不能为空"
		return
	}
	if u.isNew && len([]rune(u.password)) < 6 {
		u.formErr = "密码至少 6 位"
		return
	}
	if !u.isNew && u.password != "" && len([]rune(u.password)) < 6 {
		u.formErr = "密码至少 6 位"
		return
	}
	u.formErr, u.busy = "", true
	draft, password, isNew := u.draft, u.password, u.isNew
	win, repo := u.win, u.repo
	go func() {
		ctx := context.Background()
		var err error
		if isNew {
			err = repo.Users().Create(ctx, &draft, password)
		} else {
			err = repo.Users().Update(ctx, draft)
			if err == nil && password != "" {
				err = repo.Users().SetPassword(ctx, draft.ID, password)
			}
		}
		win.Update(func() {
			u.busy = false
			if err != nil {
				u.formErr = err.Error()
				return
			}
			u.editing, u.password = false, ""
			if draft.ID == u.self.ID && u.onSelf != nil {
				u.onSelf(draft)
			}
			u.reload()
		})
	}()
}

func (u *Users) remove(id int64) {
	u.busy = true
	win, repo := u.win, u.repo
	go func() {
		err := repo.Users().Delete(context.Background(), id)
		win.Update(func() {
			u.busy = false
			if err != nil {
				u.err = err
				return
			}
			u.editing, u.sel = false, -1
			u.reload()
		})
	}()
}

func (u *Users) View(c *ui.Context) {
	if !u.loaded && !u.busy {
		u.reload()
	}
	design.Page(c, design.PageProps{
		Title:    "账号管理",
		Subtitle: "谁能登录、能做什么，都由这里的角色决定",
		Actions: func() {
			if ui.Button(c, "刷新").Clicked() {
				u.reload()
			}
			if ui.PrimaryButton(c, "新增账号").Clicked() {
				u.startNew()
			}
		},
	}, func() {
		design.ErrorBar(c, u.err)
		ui.Row(c).Fill().Grow(1).MinHeight(0).Gap(design.Gap).AlignItems(ui.Stretch).Children(func() {
			u.listPane(c)
			if u.editing {
				u.editor(c)
			}
		})
	})
	u.confirmDelete(c)
}

func (u *Users) listPane(c *ui.Context) {
	t := design.Theme(c)
	design.Card(c, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "账号").FontSize(13).Bold().Grow(1)
			if u.busy {
				ui.Spinner(c).Width(14)
			}
			ui.Textf(c, "%d 个", len(u.rows)).FontSize(12).TextColor(t.TextMuted)
		})
		if len(u.rows) == 0 {
			if u.busy {
				design.Loading(c, "账号")
			} else {
				design.Empty(c, "还没有账号", "点右上角新增，给同事一个能登录的账号。")
			}
			return
		}
		cols := []ui.TableColumn{
			{Title: "用户名", Width: 140},
			{Title: "显示名", Width: 120},
			{Title: "角色", Width: 100},
			{Title: "创建时间", Width: 130},
			{Title: "说明", Width: 0},
		}
		u.tbl.Key = func(row int) any { return u.rows[row].ID }
		u.tbl.Label = func(row int) string { return u.rows[row].Username }
		u.tbl.Selected = &u.sel
		if design.Table(c, &u.tbl, cols, len(u.rows), func(row, col int) {
			v := u.rows[row]
			switch col {
			case 0:
				ui.Text(c, v.Username+selfMark(v, u.self)).SingleLine()
			case 1:
				ui.Text(c, v.DisplayName).SingleLine()
			case 2:
				design.Pill(c, domain.ParseRole(v.Role).Label(), roleTone(domain.ParseRole(v.Role)))
			case 3:
				ui.Text(c, design.DateTime(v.CreatedAt)).SingleLine()
			case 4:
				ui.Text(c, roleNote(domain.ParseRole(v.Role))).FontSize(11).
					TextColor(t.TextMuted).SingleLine()
			}
		}).Grow(1).Submitted() {
			if u.sel >= 0 && u.sel < len(u.rows) {
				u.edit(u.rows[u.sel])
			}
		}
		ui.Text(c, "双击一行修改角色或重置密码；「(我)」是你自己的账号").
			FontSize(11).TextColor(t.TextMuted)
	}).Grow(1).MinWidth(0)
}

func (u *Users) editor(c *ui.Context) {
	t := design.Theme(c)
	title := "编辑账号"
	if u.isNew {
		title = "新增账号"
	}
	design.Card(c, func() {
		ui.Text(c, title).FontSize(14).Bold()
		ui.Field(c, "用户名", func() {
			ui.TextInput(c, &u.draft.Username).Disabled(!u.isNew).Placeholder("登录名，如 wanglaoshi")
		})
		design.TextField(c, "显示名", &u.draft.DisplayName, "如 王老师")
		u.rolePicker(c)
		label := "新密码（留空不改）"
		if u.isNew {
			label = "密码"
		}
		ui.Field(c, label, func() {
			ui.TextInput(c, &u.password).Password().Placeholder("至少 6 位")
		})
		ui.Text(c, roleNote(domain.ParseRole(u.draft.Role))).FontSize(11).TextColor(t.TextMuted)
		design.FormError(c, u.formErr)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if !u.isNew && u.draft.ID != u.self.ID && ui.Button(c, "删除").Clicked() {
				u.askingDelete = true
			}
			ui.Spacer(c)
			design.FormActions(c, func() { u.editing = false }, u.save, "保存")
		})
	}).Width(design.ToolbarWidth).Shrink(0)
}

// rolePicker is the three roles as a segmented control, with the note of
// the chosen one under it.
func (u *Users) rolePicker(c *ui.Context) {
	labels := make([]string, len(domain.Roles))
	chosen := 0
	for i, r := range domain.Roles {
		labels[i] = r.Label()
		if string(r) == u.draft.Role {
			chosen = i
		}
	}
	ui.Field(c, "角色", func() {
		if ui.Segmented(c, &chosen, labels...).Changed() {
			u.draft.Role = string(domain.Roles[chosen])
		}
	})
}

func (u *Users) confirmDelete(c *ui.Context) {
	if !design.Confirm(c, &u.askingDelete, "删除这个账号？",
		fmt.Sprintf("「%s」将无法再登录，历史记录不受影响。", u.draft.Username), "删除") {
		return
	}
	if u.draft.ID != 0 {
		u.remove(u.draft.ID)
	}
}

func selfMark(v, self domain.User) string {
	if v.ID == self.ID {
		return "（我）"
	}
	return ""
}

func roleTone(r domain.Role) design.Tone {
	switch r {
	case domain.RoleAdmin:
		return design.ToneAccent
	case domain.RoleStaff:
		return design.TonePositive
	default:
		return design.ToneNeutral
	}
}

func roleNote(r domain.Role) string {
	switch r {
	case domain.RoleAdmin:
		return "全部权限，包括系统设置与账号管理"
	case domain.RoleStaff:
		return "学生、老师、排课、充值消课可读写；不能进系统设置"
	case domain.RoleViewer:
		return "只能查看业务数据，不能新增或修改"
	}
	return ""
}
