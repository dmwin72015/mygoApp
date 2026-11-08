package features

import (
	"context"
	"fmt"
	"strings"

	"mygo-app/internal/design"
	"mygo-app/internal/domain"

	"github.com/egoist/mygo/ui"
)

// Classrooms is 教室管理: the list on the left, the editor of the chosen
// room on the right.
type Classrooms struct {
	repo     domain.Repo
	win      design.Updater
	canWrite bool

	rows   []domain.Classroom
	loaded bool
	busy   bool
	err    error

	q     string
	lastQ string
	tbl   ui.ListState
	sel   int

	editing bool
	isNew   bool
	draft   domain.Classroom
	formErr string

	askingDelete bool
}

// NewClassrooms builds the 教室管理 page.
func NewClassrooms(repo domain.Repo, win design.Updater, canWrite bool) *Classrooms {
	return &Classrooms{repo: repo, win: win, canWrite: canWrite, sel: -1}
}

func (cr *Classrooms) reload() {
	cr.busy = true
	query := domain.Query{Search: strings.TrimSpace(cr.q)}
	win, repo := cr.win, cr.repo
	go func() {
		rows, err := repo.Classrooms().List(context.Background(), query)
		win.Update(func() {
			cr.rows, cr.err, cr.busy, cr.loaded = rows, err, false, true
			if cr.sel >= len(cr.rows) {
				cr.sel = len(cr.rows) - 1
			}
			if cr.sel < -1 {
				cr.sel = -1
			}
		})
	}()
}

func (cr *Classrooms) startNew() {
	cr.draft = domain.Classroom{Capacity: 20}
	cr.isNew, cr.editing, cr.formErr = true, true, ""
}

func (cr *Classrooms) edit(v domain.Classroom) {
	cr.draft = v
	cr.isNew, cr.editing, cr.formErr = false, true, ""
}

func (cr *Classrooms) save() {
	cr.draft.Name = strings.TrimSpace(cr.draft.Name)
	if cr.draft.Name == "" {
		cr.formErr = "教室名称不能为空"
		return
	}
	if cr.draft.Capacity <= 0 {
		cr.formErr = "教室容量必须大于 0"
		return
	}
	cr.formErr = ""
	cr.busy = true
	draft, win, repo := cr.draft, cr.win, cr.repo
	go func() {
		err := repo.Classrooms().Save(context.Background(), &draft)
		win.Update(func() {
			cr.busy = false
			if err != nil {
				cr.formErr = err.Error()
				return
			}
			cr.editing = false
			cr.reload()
		})
	}()
}

func (cr *Classrooms) remove(id string) {
	cr.busy = true
	win, repo := cr.win, cr.repo
	go func() {
		err := repo.Classrooms().Delete(context.Background(), id)
		win.Update(func() {
			cr.busy = false
			if err != nil {
				cr.err = err
				return
			}
			cr.editing = false
			cr.sel = -1
			cr.reload()
		})
	}()
}

// View builds the page.
func (cr *Classrooms) View(c *ui.Context) {
	if !cr.loaded && !cr.busy {
		cr.reload()
	}
	if cr.q != cr.lastQ {
		cr.lastQ = cr.q
		cr.reload()
	}

	design.Page(c, design.PageProps{
		Title:    "教室管理",
		Subtitle: fmt.Sprintf("%d 间教室", len(cr.rows)),
		Actions: func() {
			if ui.Button(c, "刷新").Clicked() {
				cr.reload()
			}
			if cr.canWrite && ui.PrimaryButton(c, "新增教室").Clicked() {
				cr.startNew()
			}
		},
	}, func() {
		design.ErrorBar(c, cr.err)
		ui.Row(c).Fill().Grow(1).MinHeight(0).Gap(design.Gap).AlignItems(ui.Stretch).Children(func() {
			cr.listPane(c)
			if cr.editing {
				cr.editor(c)
			}
		})
	})

	cr.confirmDelete(c)
}

func (cr *Classrooms) listPane(c *ui.Context) {
	theme := design.Theme(c)
	design.Card(c, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.SearchField(c, &cr.q).Width(240).Placeholder("搜索教室名称或位置")
			ui.Spacer(c)
			if cr.busy {
				ui.Spinner(c).Width(14)
			}
			ui.Textf(c, "%d 间", len(cr.rows)).FontSize(12).TextColor(theme.TextMuted)
		})
		if len(cr.rows) == 0 {
			if cr.busy {
				design.Loading(c, "教室")
			} else {
				design.Empty(c, "还没有教室", "点右上角「新增教室」录入场地信息，排课时即可选用。")
			}
			return
		}
		cols := []ui.TableColumn{
			{Title: "教室名称", Width: 140, Sortable: true},
			{Title: "容纳人数", Width: 90, Align: ui.End},
			{Title: "位置", Width: 160},
			{Title: "备注", Width: 200},
		}
		cr.tbl.Key = func(row int) any { return cr.rows[row].ID }
		cr.tbl.Label = func(row int) string { return cr.rows[row].Name }
		cr.tbl.Selected = &cr.sel
		if design.Table(c, &cr.tbl, cols, len(cr.rows), func(row, col int) {
			v := cr.rows[row]
			switch col {
			case 0:
				ui.Text(c, v.Name).SingleLine()
			case 1:
				ui.Textf(c, "%d 人", v.Capacity).SingleLine()
			case 2:
				ui.Text(c, v.Location).SingleLine()
			case 3:
				ui.Text(c, v.Note).SingleLine().TextColor(theme.TextMuted)
			}
		}).Grow(1).Submitted() {
			if cr.canWrite && cr.sel >= 0 && cr.sel < len(cr.rows) {
				cr.edit(cr.rows[cr.sel])
			}
		}
		hint := "双击一行进行编辑"
		if !cr.canWrite {
			hint = "只读账号：可以查看教室列表，不能新增或修改"
		}
		ui.Text(c, hint).FontSize(11).TextColor(theme.TextMuted)
	}).Grow(1).MinWidth(0)
}

func (cr *Classrooms) editor(c *ui.Context) {
	title := "编辑教室"
	if cr.isNew {
		title = "新增教室"
	}
	design.Card(c, func() {
		ui.Text(c, title).FontSize(14).Bold()
		design.TextField(c, "教室名称", &cr.draft.Name, "必填，如 一号琴房 / 多媒体大教室")
		design.IntField(c, "容纳人数", &cr.draft.Capacity, 1, 500)
		design.TextField(c, "位置", &cr.draft.Location, "如 教学楼 2 楼 201")
		design.TextAreaField(c, "备注", &cr.draft.Note, "设备配置、场地特点等")
		design.FormError(c, cr.formErr)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if !cr.isNew && ui.Button(c, "删除").Clicked() {
				cr.askingDelete = true
			}
			ui.Spacer(c)
			design.FormActions(c, func() { cr.editing = false }, cr.save, "保存")
		})
	}).Width(design.ToolbarWidth).Shrink(0)
}

func (cr *Classrooms) confirmDelete(c *ui.Context) {
	if !design.Confirm(c, &cr.askingDelete, "删除该教室？", "删除后将无法在此教室安排新排课，操作不可撤销。", "删除") {
		return
	}
	if cr.draft.ID != "" {
		cr.remove(cr.draft.ID)
	}
}
