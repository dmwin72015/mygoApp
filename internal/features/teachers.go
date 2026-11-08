package features

import (
	"context"
	"fmt"
	"strings"

	"mygo-app/internal/design"
	"mygo-app/internal/domain"

	"github.com/egoist/mygo/ui"
)

// Teachers is 老师管理: the list on the left, the editor of the chosen
// teacher on the right.
type Teachers struct {
	repo     domain.Repo
	win      design.Updater
	canWrite bool

	rows   []domain.Teacher
	loaded bool
	busy   bool
	err    error

	q     string
	lastQ string
	tbl   ui.ListState
	sel   int

	editing bool
	isNew   bool
	draft   domain.Teacher
	formErr string

	askingDelete bool
}

// NewTeachers builds the 老师管理 page.
func NewTeachers(repo domain.Repo, win design.Updater, canWrite bool) *Teachers {
	return &Teachers{repo: repo, win: win, canWrite: canWrite, sel: -1}
}

func (t *Teachers) reload() {
	t.busy = true
	query := domain.Query{Search: strings.TrimSpace(t.q)}
	win, repo := t.win, t.repo
	go func() {
		rows, err := repo.Teachers().List(context.Background(), query)
		win.Update(func() {
			t.rows, t.err, t.busy, t.loaded = rows, err, false, true
			if t.sel >= len(t.rows) {
				t.sel = len(t.rows) - 1
			}
			if t.sel < -1 {
				t.sel = -1
			}
		})
	}()
}

func (t *Teachers) startNew() {
	t.draft = domain.Teacher{Active: true, RateCents: 20000} // 默认 200 元/课时
	t.isNew, t.editing, t.formErr = true, true, ""
}

func (t *Teachers) edit(v domain.Teacher) {
	t.draft = v
	t.isNew, t.editing, t.formErr = false, true, ""
}

func (t *Teachers) save() {
	t.draft.Name = strings.TrimSpace(t.draft.Name)
	if t.draft.Name == "" {
		t.formErr = "姓名不能为空"
		return
	}
	t.formErr = ""
	t.busy = true
	draft, win, repo := t.draft, t.win, t.repo
	go func() {
		err := repo.Teachers().Save(context.Background(), &draft)
		win.Update(func() {
			t.busy = false
			if err != nil {
				t.formErr = err.Error()
				return
			}
			t.editing = false
			t.reload()
		})
	}()
}

func (t *Teachers) remove(id string) {
	t.busy = true
	win, repo := t.win, t.repo
	go func() {
		err := repo.Teachers().Delete(context.Background(), id)
		win.Update(func() {
			t.busy = false
			if err != nil {
				t.err = err
				return
			}
			t.editing = false
			t.sel = -1
			t.reload()
		})
	}()
}

// View builds the page.
func (t *Teachers) View(c *ui.Context) {
	if !t.loaded && !t.busy {
		t.reload()
	}
	if t.q != t.lastQ {
		t.lastQ = t.q
		t.reload()
	}

	design.Page(c, design.PageProps{
		Title:    "老师管理",
		Subtitle: fmt.Sprintf("%d 位老师", len(t.rows)),
		Actions: func() {
			if ui.Button(c, "刷新").Clicked() {
				t.reload()
			}
			if t.canWrite && ui.PrimaryButton(c, "新增老师").Clicked() {
				t.startNew()
			}
		},
	}, func() {
		design.ErrorBar(c, t.err)
		ui.Row(c).Fill().Grow(1).MinHeight(0).Gap(design.Gap).AlignItems(ui.Stretch).Children(func() {
			t.listPane(c)
			if t.editing {
				t.editor(c)
			}
		})
	})

	t.confirmDelete(c)
}

func (t *Teachers) listPane(c *ui.Context) {
	theme := design.Theme(c)
	design.Card(c, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.SearchField(c, &t.q).Width(240).Placeholder("搜索姓名或科目")
			ui.Spacer(c)
			if t.busy {
				ui.Spinner(c).Width(14)
			}
			ui.Textf(c, "%d 位", len(t.rows)).FontSize(12).TextColor(theme.TextMuted)
		})
		if len(t.rows) == 0 {
			if t.busy {
				design.Loading(c, "老师")
			} else {
				design.Empty(c, "还没有老师", "点右上角「新增老师」添加师资档案。")
			}
			return
		}
		cols := []ui.TableColumn{
			{Title: "姓名", Width: 120, Sortable: true},
			{Title: "科目", Width: 100},
			{Title: "电话", Width: 130},
			{Title: "课时费", Width: 100, Align: ui.End},
			{Title: "状态", Width: 80},
			{Title: "入职日期", Width: 100},
		}
		t.tbl.Key = func(row int) any { return t.rows[row].ID }
		t.tbl.Label = func(row int) string { return t.rows[row].Name }
		t.tbl.Selected = &t.sel
		if design.Table(c, &t.tbl, cols, len(t.rows), func(row, col int) {
			v := t.rows[row]
			switch col {
			case 0:
				ui.Text(c, v.Name).SingleLine()
			case 1:
				ui.Text(c, v.Subject).SingleLine()
			case 2:
				ui.Text(c, v.Phone).SingleLine()
			case 3:
				ui.Text(c, design.Money(v.RateCents)).SingleLine()
			case 4:
				design.Pill(c, teacherStatusLabel(v.Active), activeTone(v.Active))
			case 5:
				ui.Text(c, design.Date(v.CreatedAt)).SingleLine()
			}
		}).Grow(1).Submitted() {
			if t.canWrite && t.sel >= 0 && t.sel < len(t.rows) {
				t.edit(t.rows[t.sel])
			}
		}
		hint := "双击一行进行编辑"
		if !t.canWrite {
			hint = "只读账号：可以查看老师列表，不能新增或修改"
		}
		ui.Text(c, hint).FontSize(11).TextColor(theme.TextMuted)
	}).Grow(1).MinWidth(0)
}

func (t *Teachers) editor(c *ui.Context) {
	title := "编辑老师"
	if t.isNew {
		title = "新增老师"
	}
	design.Card(c, func() {
		ui.Text(c, title).FontSize(14).Bold()
		design.TextField(c, "姓名", &t.draft.Name, "必填")
		design.TextField(c, "科目", &t.draft.Subject, "教学科目，如 钢琴 / 英语")
		design.TextField(c, "联系电话", &t.draft.Phone, "手机号码")
		design.MoneyField(c, "课时费", &t.draft.RateCents)
		design.SwitchField(c, "在职状态", &t.draft.Active)
		design.FormError(c, t.formErr)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if !t.isNew && ui.Button(c, "删除").Clicked() {
				t.askingDelete = true
			}
			ui.Spacer(c)
			design.FormActions(c, func() { t.editing = false }, t.save, "保存")
		})
	}).Width(design.ToolbarWidth).Shrink(0)
}

func (t *Teachers) confirmDelete(c *ui.Context) {
	if !design.Confirm(c, &t.askingDelete, "删除该老师？", "删除后将无法为该老师指派新课程，操作不可撤销。", "删除") {
		return
	}
	if t.draft.ID != "" {
		t.remove(t.draft.ID)
	}
}

func teacherStatusLabel(active bool) string {
	if active {
		return "在职"
	}
	return "离职"
}
