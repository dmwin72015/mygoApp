package features

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"mygo-app/internal/design"
	"mygo-app/internal/domain"

	"github.com/egoist/mygo/ui"
)

// Students is 学生管理: the list on the left, the editor of the chosen
// student on the right. It is the reference implementation of a CRUD
// page — copy this file for the next entity that needs writing, not just
// reading.
//
// State lives here, not in the view: the view runs every frame, so a query
// belongs in reload, never in View.
type Students struct {
	repo domain.Repo
	win  design.Updater
	// canWrite is false for a role that only reads: the page hides every
	// control that would change a student, and the rows open nothing.
	canWrite bool

	rows   []domain.Student
	loaded bool
	busy   bool
	err    error

	q     string
	lastQ string
	tbl   ui.ListState
	sel   int

	editing bool
	isNew   bool
	draft   domain.Student
	formErr string

	askingDelete bool
}

// NewStudents builds the page. It loads on its first frame; canWrite is
// what the signed-in role may do here.
func NewStudents(repo domain.Repo, win design.Updater, canWrite bool) *Students {
	return &Students{repo: repo, win: win, canWrite: canWrite, sel: -1}
}

// reload reads the list off the main thread and hands it back through
// Window.Update, so the UI updates when the rows are in.
func (s *Students) reload() {
	s.busy = true
	query := domain.Query{Search: strings.TrimSpace(s.q)}
	win, repo := s.win, s.repo
	go func() {
		rows, err := repo.Students().List(context.Background(), query)
		win.Update(func() {
			s.rows, s.err, s.busy, s.loaded = rows, err, false, true
			if s.sel >= len(s.rows) {
				s.sel = len(s.rows) - 1
			}
			if s.sel < -1 {
				s.sel = -1
			}
		})
	}()
}

func (s *Students) startNew() {
	s.draft = domain.Student{Active: true}
	s.isNew, s.editing, s.formErr = true, true, ""
}

func (s *Students) edit(v domain.Student) {
	s.draft = v
	s.isNew, s.editing, s.formErr = false, true, ""
}

func (s *Students) save() {
	s.draft.Name = strings.TrimSpace(s.draft.Name)
	if s.draft.Name == "" {
		s.formErr = "姓名不能为空"
		return
	}
	s.formErr = ""
	s.busy = true
	draft, win, repo := s.draft, s.win, s.repo
	go func() {
		err := repo.Students().Save(context.Background(), &draft)
		win.Update(func() {
			s.busy = false
			if err != nil {
				s.formErr = err.Error()
				return
			}
			s.editing = false
			s.reload()
		})
	}()
}

func (s *Students) remove(id int64) {
	s.busy = true
	win, repo := s.win, s.repo
	go func() {
		err := repo.Students().Delete(context.Background(), strconv.FormatInt(id, 10))
		win.Update(func() {
			s.busy = false
			if err != nil {
				s.err = err
				return
			}
			s.editing = false
			s.sel = -1
			s.reload()
		})
	}()
}

// View builds the page.
func (s *Students) View(c *ui.Context) {
	if !s.loaded && !s.busy {
		s.reload()
	}
	// A new search is a new query: the field writes to s.q as it types.
	if s.q != s.lastQ {
		s.lastQ = s.q
		s.reload()
	}

	var total int64
	for _, v := range s.rows {
		total += v.Balance
	}

	design.Page(c, design.PageProps{
		Title:    "学生管理",
		Subtitle: fmt.Sprintf("%d 名学生 · 余额合计 %s", len(s.rows), design.Money(total)),
		Actions: func() {
			if ui.Button(c, "刷新").Clicked() {
				s.reload()
			}
			if s.canWrite && ui.PrimaryButton(c, "新增学生").Clicked() {
				s.startNew()
			}
		},
	}, func() {
		design.ErrorBar(c, s.err)
		ui.Row(c).Fill().Grow(1).MinHeight(0).Gap(design.Gap).AlignItems(ui.Stretch).Children(func() {
			s.listPane(c)
			if s.editing {
				s.editor(c)
			}
		})
	})

	s.confirmDelete(c)
}

func (s *Students) listPane(c *ui.Context) {
	t := design.Theme(c)
	design.Card(c, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.SearchField(c, &s.q).Width(240).Placeholder("搜索姓名、家长或电话")
			ui.Spacer(c)
			if s.busy {
				ui.Spinner(c).Width(14)
			}
			ui.Textf(c, "%d 条", len(s.rows)).FontSize(12).TextColor(t.TextMuted)
		})
		if len(s.rows) == 0 {
			if s.busy {
				design.Loading(c, "学生")
			} else {
				design.Empty(c, "还没有学生", "点右上角「新增学生」建档，之后就能充值排课。")
			}
			return
		}
		cols := []ui.TableColumn{
			{Title: "姓名", Width: 120, Sortable: true},
			{Title: "家长", Width: 100},
			{Title: "电话", Width: 130},
			{Title: "余额", Width: 110, Align: ui.End},
			{Title: "状态", Width: 72},
			{Title: "建档日期", Width: 100},
		}
		s.tbl.Key = func(row int) any { return s.rows[row].ID }
		s.tbl.Label = func(row int) string { return s.rows[row].Name }
		s.tbl.Selected = &s.sel
		if design.Table(c, &s.tbl, cols, len(s.rows), func(row, col int) {
			v := s.rows[row]
			switch col {
			case 0:
				ui.Text(c, v.Name).SingleLine()
			case 1:
				ui.Text(c, v.Guardian).SingleLine()
			case 2:
				ui.Text(c, v.Phone).SingleLine()
			case 3:
				ui.Text(c, design.Money(v.Balance)).SingleLine()
			case 4:
				design.Pill(c, activeLabel(v.Active), activeTone(v.Active))
			case 5:
				ui.Text(c, design.Date(v.CreatedAt)).SingleLine()
			}
		}).Grow(1).Submitted() {
			if s.canWrite && s.sel >= 0 && s.sel < len(s.rows) {
				s.edit(s.rows[s.sel])
			}
		}
		hint := "双击一行编辑，余额通过「充值 / 消课」变动"
		if !s.canWrite {
			hint = "只读账号：可以查看学生，不能新增或修改"
		}
		ui.Text(c, hint).FontSize(11).TextColor(t.TextMuted)
	}).Grow(1).MinWidth(0)
}

func (s *Students) editor(c *ui.Context) {
	t := design.Theme(c)
	title := "编辑学生"
	if s.isNew {
		title = "新增学生"
	}
	design.Card(c, func() {
		ui.Text(c, title).FontSize(14).Bold()
		if !s.isNew {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "当前余额").FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, design.Money(s.draft.Balance)).FontSize(13).Bold()
			})
		}
		design.TextField(c, "姓名", &s.draft.Name, "必填")
		design.TextField(c, "家长", &s.draft.Guardian, "家长姓名")
		design.TextField(c, "联系电话", &s.draft.Phone, "手机号")
		design.TextAreaField(c, "备注", &s.draft.Note, "上课时间、接送要求等")
		design.SwitchField(c, "在读", &s.draft.Active)
		design.FormError(c, s.formErr)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if !s.isNew && ui.Button(c, "删除").Clicked() {
				s.askingDelete = true
			}
			ui.Spacer(c)
			design.FormActions(c, func() { s.editing = false }, s.save, "保存")
		})
	}).Width(design.ToolbarWidth).Shrink(0)
}

func (s *Students) confirmDelete(c *ui.Context) {
	if !design.Confirm(c, &s.askingDelete, "删除该学生？", "档案会从列表中移除，操作不可撤销。", "删除") {
		return
	}
	if s.draft.ID != 0 {
		s.remove(s.draft.ID)
	}
}

func activeLabel(on bool) string {
	if on {
		return "在读"
	}
	return "停用"
}

func activeTone(on bool) design.Tone {
	if on {
		return design.TonePositive
	}
	return design.ToneNeutral
}
