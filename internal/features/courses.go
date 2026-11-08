package features

import (
	"context"
	"fmt"
	"strings"

	"mygo-app/internal/design"
	"mygo-app/internal/domain"

	"github.com/egoist/mygo/ui"
)

// Courses is 课程管理: the list on the left, the editor of the chosen
// course on the right.
type Courses struct {
	repo     domain.Repo
	win      design.Updater
	canWrite bool

	rows         []domain.Course
	teacherNames map[string]string // ID -> Name
	teacherIDs   map[string]string // Name -> ID
	teacherOpts  []string          // options for SelectField

	loaded bool
	busy   bool
	err    error

	q     string
	lastQ string
	tbl   ui.ListState
	sel   int

	editing         bool
	isNew           bool
	draft           domain.Course
	selectedTeacher string
	formErr         string

	askingDelete bool
}

// NewCourses builds the 课程管理 page.
func NewCourses(repo domain.Repo, win design.Updater, canWrite bool) *Courses {
	return &Courses{
		repo:         repo,
		win:          win,
		canWrite:     canWrite,
		sel:          -1,
		teacherNames: make(map[string]string),
		teacherIDs:   make(map[string]string),
		teacherOpts:  []string{"未指定"},
	}
}

func (c *Courses) reload() {
	c.busy = true
	query := domain.Query{Search: strings.TrimSpace(c.q)}
	win, repo := c.win, c.repo
	go func() {
		ctx := context.Background()
		rows, err := repo.Courses().List(ctx, query)
		teachers, _ := repo.Teachers().List(ctx, domain.Query{})

		tNames := make(map[string]string, len(teachers))
		tIDs := make(map[string]string, len(teachers))
		opts := []string{"未指定"}
		for _, t := range teachers {
			tNames[t.ID] = t.Name
			tIDs[t.Name] = t.ID
			opts = append(opts, t.Name)
		}

		win.Update(func() {
			c.rows, c.err, c.busy, c.loaded = rows, err, false, true
			c.teacherNames = tNames
			c.teacherIDs = tIDs
			c.teacherOpts = opts
			if c.sel >= len(c.rows) {
				c.sel = len(c.rows) - 1
			}
			if c.sel < -1 {
				c.sel = -1
			}
		})
	}()
}

func (c *Courses) startNew() {
	c.draft = domain.Course{
		Active:       true,
		TotalLessons: 10,
		Minutes:      45,
		PriceCents:   120000, // 默认 1200 元
	}
	c.selectedTeacher = "未指定"
	c.isNew, c.editing, c.formErr = true, true, ""
}

func (c *Courses) edit(v domain.Course) {
	c.draft = v
	if name, ok := c.teacherNames[v.TeacherID]; ok && name != "" {
		c.selectedTeacher = name
	} else {
		c.selectedTeacher = "未指定"
	}
	c.isNew, c.editing, c.formErr = false, true, ""
}

func (c *Courses) save() {
	c.draft.Name = strings.TrimSpace(c.draft.Name)
	if c.draft.Name == "" {
		c.formErr = "课程名称不能为空"
		return
	}
	if c.draft.TotalLessons <= 0 {
		c.formErr = "课时数必须大于 0"
		return
	}
	if c.draft.Minutes <= 0 {
		c.formErr = "单节时长必须大于 0"
		return
	}
	if c.selectedTeacher != "" && c.selectedTeacher != "未指定" {
		c.draft.TeacherID = c.teacherIDs[c.selectedTeacher]
	} else {
		c.draft.TeacherID = ""
	}

	c.formErr = ""
	c.busy = true
	draft, win, repo := c.draft, c.win, c.repo
	go func() {
		err := repo.Courses().Save(context.Background(), &draft)
		win.Update(func() {
			c.busy = false
			if err != nil {
				c.formErr = err.Error()
				return
			}
			c.editing = false
			c.reload()
		})
	}()
}

func (c *Courses) remove(id string) {
	c.busy = true
	win, repo := c.win, c.repo
	go func() {
		err := repo.Courses().Delete(context.Background(), id)
		win.Update(func() {
			c.busy = false
			if err != nil {
				c.err = err
				return
			}
			c.editing = false
			c.sel = -1
			c.reload()
		})
	}()
}

// View builds the page.
func (c *Courses) View(ctx *ui.Context) {
	if !c.loaded && !c.busy {
		c.reload()
	}
	if c.q != c.lastQ {
		c.lastQ = c.q
		c.reload()
	}

	design.Page(ctx, design.PageProps{
		Title:    "课程管理",
		Subtitle: fmt.Sprintf("%d 门课程", len(c.rows)),
		Actions: func() {
			if ui.Button(ctx, "刷新").Clicked() {
				c.reload()
			}
			if c.canWrite && ui.PrimaryButton(ctx, "新增课程").Clicked() {
				c.startNew()
			}
		},
	}, func() {
		design.ErrorBar(ctx, c.err)
		ui.Row(ctx).Fill().Grow(1).MinHeight(0).Gap(design.Gap).AlignItems(ui.Stretch).Children(func() {
			c.listPane(ctx)
			if c.editing {
				c.editor(ctx)
			}
		})
	})

	c.confirmDelete(ctx)
}

func (c *Courses) listPane(ctx *ui.Context) {
	theme := design.Theme(ctx)
	design.Card(ctx, func() {
		ui.Row(ctx).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.SearchField(ctx, &c.q).Width(240).Placeholder("搜索课程名称或科目")
			ui.Spacer(ctx)
			if c.busy {
				ui.Spinner(ctx).Width(14)
			}
			ui.Textf(ctx, "%d 门", len(c.rows)).FontSize(12).TextColor(theme.TextMuted)
		})
		if len(c.rows) == 0 {
			if c.busy {
				design.Loading(ctx, "课程")
			} else {
				design.Empty(ctx, "还没有课程", "点右上角「新增课程」配置课程包与课时。")
			}
			return
		}
		cols := []ui.TableColumn{
			{Title: "课程名称", Width: 200, Sortable: true},
			{Title: "科目", Width: 90},
			{Title: "任课老师", Width: 110},
			{Title: "课时数", Width: 80, Align: ui.End},
			{Title: "单节时长", Width: 90, Align: ui.End},
			{Title: "价格", Width: 100, Align: ui.End},
			{Title: "状态", Width: 80},
		}
		c.tbl.Key = func(row int) any { return c.rows[row].ID }
		c.tbl.Label = func(row int) string { return c.rows[row].Name }
		c.tbl.Selected = &c.sel
		if design.Table(ctx, &c.tbl, cols, len(c.rows), func(row, col int) {
			v := c.rows[row]
			switch col {
			case 0:
				ui.Text(ctx, v.Name).SingleLine()
			case 1:
				ui.Text(ctx, v.Subject).SingleLine()
			case 2:
				teacherName := c.teacherNames[v.TeacherID]
				if teacherName == "" {
					teacherName = "未指定"
				}
				ui.Text(ctx, teacherName).SingleLine()
			case 3:
				ui.Textf(ctx, "%d 节", v.TotalLessons).SingleLine()
			case 4:
				ui.Textf(ctx, "%d 分钟", v.Minutes).SingleLine()
			case 5:
				ui.Text(ctx, design.Money(v.PriceCents)).SingleLine()
			case 6:
				design.Pill(ctx, courseStatusLabel(v.Active), activeTone(v.Active))
			}
		}).Grow(1).Submitted() {
			if c.canWrite && c.sel >= 0 && c.sel < len(c.rows) {
				c.edit(c.rows[c.sel])
			}
		}
		hint := "双击一行进行编辑"
		if !c.canWrite {
			hint = "只读账号：可以查看课程列表，不能新增或修改"
		}
		ui.Text(ctx, hint).FontSize(11).TextColor(theme.TextMuted)
	}).Grow(1).MinWidth(0)
}

func (c *Courses) editor(ctx *ui.Context) {
	title := "编辑课程"
	if c.isNew {
		title = "新增课程"
	}
	design.Card(ctx, func() {
		ui.Text(ctx, title).FontSize(14).Bold()
		design.TextField(ctx, "课程名称", &c.draft.Name, "必填，如 少儿钢琴初级班")
		design.TextField(ctx, "科目", &c.draft.Subject, "教学科目，如 钢琴 / 英语")
		if len(c.teacherOpts) > 0 {
			design.SelectField(ctx, "任课老师", &c.selectedTeacher, c.teacherOpts)
		}
		design.IntField(ctx, "总课时数", &c.draft.TotalLessons, 1, 1000)
		design.IntField(ctx, "单节时长 (分钟)", &c.draft.Minutes, 15, 360)
		design.MoneyField(ctx, "课程总价", &c.draft.PriceCents)
		design.SwitchField(ctx, "上架销售", &c.draft.Active)
		design.FormError(ctx, c.formErr)
		ui.Row(ctx).Gap(8).Justify(ui.End).Children(func() {
			if !c.isNew && ui.Button(ctx, "删除").Clicked() {
				c.askingDelete = true
			}
			ui.Spacer(ctx)
			design.FormActions(ctx, func() { c.editing = false }, c.save, "保存")
		})
	}).Width(design.ToolbarWidth).Shrink(0)
}

func (c *Courses) confirmDelete(ctx *ui.Context) {
	if !design.Confirm(ctx, &c.askingDelete, "删除该课程？", "已售出该课程的学员记录将保留，但无法再被排课选用。", "删除") {
		return
	}
	if c.draft.ID != "" {
		c.remove(c.draft.ID)
	}
}

func courseStatusLabel(active bool) string {
	if active {
		return "上架"
	}
	return "下架"
}
