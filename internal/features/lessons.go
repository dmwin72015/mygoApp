package features

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mygo-app/internal/design"
	"mygo-app/internal/domain"

	"github.com/egoist/mygo/ui"
)

// Lessons is 排课: every lesson with the names its IDs stand for.
type Lessons struct {
	repo     domain.Repo
	win      design.Updater
	canWrite bool

	rows        []domain.Lesson
	courses     map[string]string // ID -> Name
	teachers    map[string]string // ID -> Name
	rooms       map[string]string // ID -> Name
	courseIDs   map[string]string // Name -> ID
	teacherIDs  map[string]string // Name -> ID
	roomIDs     map[string]string // Name -> ID
	courseOpts  []string
	teacherOpts []string
	roomOpts    []string
	statusOpts  []string

	loaded, busy bool
	err          error
	tbl          ui.ListState
	sel          int

	editing         bool
	isNew           bool
	draft           domain.Lesson
	selectedCourse  string
	selectedTeacher string
	selectedRoom    string
	selectedStatus  string
	startAtStr      string
	formErr         string

	askingDelete bool
}

// NewLessons builds the 排课 page.
func NewLessons(repo domain.Repo, win design.Updater, canWrite bool) *Lessons {
	return &Lessons{
		repo:        repo,
		win:         win,
		canWrite:    canWrite,
		sel:         -1,
		courses:     make(map[string]string),
		teachers:    make(map[string]string),
		rooms:       make(map[string]string),
		courseIDs:   make(map[string]string),
		teacherIDs:  make(map[string]string),
		roomIDs:     make(map[string]string),
		courseOpts:  []string{"未指定"},
		teacherOpts: []string{"未指定"},
		roomOpts:    []string{"未指定"},
		statusOpts:  []string{"待上课", "已完成", "已取消"},
	}
}

func (p *Lessons) reload() {
	p.busy = true
	win, repo := p.win, p.repo
	go func() {
		ctx := context.Background()
		lessons, err := repo.Lessons().List(ctx, domain.Query{})
		courses, _ := repo.Courses().List(ctx, domain.Query{})
		teachers, _ := repo.Teachers().List(ctx, domain.Query{})
		rooms, _ := repo.Classrooms().List(ctx, domain.Query{})

		cMap, cIDs, cOpts := make(map[string]string), make(map[string]string), []string{"未指定"}
		for _, v := range courses {
			cMap[v.ID] = v.Name
			cIDs[v.Name] = v.ID
			cOpts = append(cOpts, v.Name)
		}

		tMap, tIDs, tOpts := make(map[string]string), make(map[string]string), []string{"未指定"}
		for _, v := range teachers {
			tMap[v.ID] = v.Name
			tIDs[v.Name] = v.ID
			tOpts = append(tOpts, v.Name)
		}

		rMap, rIDs, rOpts := make(map[string]string), make(map[string]string), []string{"未指定"}
		for _, v := range rooms {
			rMap[v.ID] = v.Name
			rIDs[v.Name] = v.ID
			rOpts = append(rOpts, v.Name)
		}

		win.Update(func() {
			p.rows, p.err, p.busy, p.loaded = lessons, err, false, true
			p.courses, p.courseIDs, p.courseOpts = cMap, cIDs, cOpts
			p.teachers, p.teacherIDs, p.teacherOpts = tMap, tIDs, tOpts
			p.rooms, p.roomIDs, p.roomOpts = rMap, rIDs, rOpts
			if p.sel >= len(p.rows) {
				p.sel = len(p.rows) - 1
			}
			if p.sel < -1 {
				p.sel = -1
			}
		})
	}()
}

func (p *Lessons) startNew() {
	nextHour := time.Now().Add(2 * time.Hour).Truncate(time.Hour)
	p.draft = domain.Lesson{
		StartAt: nextHour,
		Minutes: 45,
		Status:  "planned",
	}
	p.selectedCourse = "未指定"
	p.selectedTeacher = "未指定"
	p.selectedRoom = "未指定"
	p.selectedStatus = "待上课"
	p.startAtStr = nextHour.Format("2006-01-02 15:04")
	p.isNew, p.editing, p.formErr = true, true, ""
}

func (p *Lessons) edit(v domain.Lesson) {
	p.draft = v
	if name, ok := p.courses[v.CourseID]; ok && name != "" {
		p.selectedCourse = name
	} else {
		p.selectedCourse = "未指定"
	}
	if name, ok := p.teachers[v.TeacherID]; ok && name != "" {
		p.selectedTeacher = name
	} else {
		p.selectedTeacher = "未指定"
	}
	if name, ok := p.rooms[v.ClassroomID]; ok && name != "" {
		p.selectedRoom = name
	} else {
		p.selectedRoom = "未指定"
	}
	switch v.Status {
	case "done":
		p.selectedStatus = "已完成"
	case "canceled":
		p.selectedStatus = "已取消"
	default:
		p.selectedStatus = "待上课"
	}
	p.startAtStr = v.StartAt.Format("2006-01-02 15:04")
	p.isNew, p.editing, p.formErr = false, true, ""
}

func (p *Lessons) save() {
	if p.selectedCourse == "" || p.selectedCourse == "未指定" {
		p.formErr = "请选择课程"
		return
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", strings.TrimSpace(p.startAtStr), time.Local)
	if err != nil {
		p.formErr = "时间格式错误，请输入如 2006-01-02 15:04"
		return
	}
	if p.draft.Minutes <= 0 {
		p.formErr = "排课时长必须大于 0"
		return
	}

	p.draft.CourseID = p.courseIDs[p.selectedCourse]
	if p.selectedTeacher != "" && p.selectedTeacher != "未指定" {
		p.draft.TeacherID = p.teacherIDs[p.selectedTeacher]
	} else {
		p.draft.TeacherID = ""
	}
	if p.selectedRoom != "" && p.selectedRoom != "未指定" {
		p.draft.ClassroomID = p.roomIDs[p.selectedRoom]
	} else {
		p.draft.ClassroomID = ""
	}
	switch p.selectedStatus {
	case "已完成":
		p.draft.Status = "done"
	case "已取消":
		p.draft.Status = "canceled"
	default:
		p.draft.Status = "planned"
	}
	p.draft.StartAt = t

	p.formErr = ""
	p.busy = true
	draft, win, repo := p.draft, p.win, p.repo
	go func() {
		err := repo.Lessons().Save(context.Background(), &draft)
		win.Update(func() {
			p.busy = false
			if err != nil {
				p.formErr = err.Error()
				return
			}
			p.editing = false
			p.reload()
		})
	}()
}

func (p *Lessons) remove(id string) {
	p.busy = true
	win, repo := p.win, p.repo
	go func() {
		err := repo.Lessons().Delete(context.Background(), id)
		win.Update(func() {
			p.busy = false
			if err != nil {
				p.err = err
				return
			}
			p.editing = false
			p.sel = -1
			p.reload()
		})
	}()
}

// View builds the page.
func (p *Lessons) View(c *ui.Context) {
	if !p.loaded && !p.busy {
		p.reload()
	}
	planned := 0
	for _, v := range p.rows {
		if v.Status == "planned" {
			planned++
		}
	}
	design.Page(c, design.PageProps{
		Title:    "排课",
		Subtitle: fmt.Sprintf("待上 %d 节 · 共 %d 节安排", planned, len(p.rows)),
		Actions: func() {
			if ui.Button(c, "刷新").Clicked() {
				p.reload()
			}
			if p.canWrite && ui.PrimaryButton(c, "新增排课").Clicked() {
				p.startNew()
			}
		},
	}, func() {
		design.ErrorBar(c, p.err)
		ui.Row(c).Fill().Grow(1).MinHeight(0).Gap(design.Gap).AlignItems(ui.Stretch).Children(func() {
			p.listPane(c)
			if p.editing {
				p.editor(c)
			}
		})
	})

	p.confirmDelete(c)
}

func (p *Lessons) listPane(c *ui.Context) {
	theme := design.Theme(c)
	design.Card(c, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "%d 节排课", len(p.rows)).FontSize(12).TextColor(theme.TextMuted).Grow(1)
			if p.busy {
				ui.Spinner(c).Width(14)
			}
		})
		if len(p.rows) == 0 {
			if p.busy {
				design.Loading(c, "排课")
			} else {
				design.Empty(c, "还没有排课", "选好课程、老师和教室，把课排进时间表。")
			}
			return
		}
		cols := []ui.TableColumn{
			{Title: "上课时间", Width: 150, Sortable: true},
			{Title: "课程", Width: 180},
			{Title: "老师", Width: 110},
			{Title: "教室", Width: 100},
			{Title: "时长", Width: 80, Align: ui.End},
			{Title: "状态", Width: 90},
		}
		p.tbl.Key = func(row int) any { return p.rows[row].ID }
		p.tbl.Selected = &p.sel
		if design.Table(c, &p.tbl, cols, len(p.rows), func(row, col int) {
			v := p.rows[row]
			switch col {
			case 0:
				ui.Text(c, design.DateTime(v.StartAt)).SingleLine()
			case 1:
				ui.Text(c, nameOr(p.courses[v.CourseID], "未命名课程")).SingleLine()
			case 2:
				ui.Text(c, nameOr(p.teachers[v.TeacherID], "未指派")).SingleLine()
			case 3:
				ui.Text(c, nameOr(p.rooms[v.ClassroomID], "未安排")).SingleLine()
			case 4:
				ui.Textf(c, "%d 分钟", v.Minutes).SingleLine()
			case 5:
				label, tone := lessonStatus(v.Status)
				design.Pill(c, label, tone)
			}
		}).Grow(1).Submitted() {
			if p.canWrite && p.sel >= 0 && p.sel < len(p.rows) {
				p.edit(p.rows[p.sel])
			}
		}
		hint := "双击一行编辑排课，可改期、改老师教室或流转状态"
		if !p.canWrite {
			hint = "只读账号：可以查看排课时间表，不能修改"
		}
		ui.Text(c, hint).FontSize(11).TextColor(theme.TextMuted)
	}).Grow(1).MinWidth(0)
}

func (p *Lessons) editor(c *ui.Context) {
	title := "调整排课"
	if p.isNew {
		title = "新增排课"
	}
	design.Card(c, func() {
		ui.Text(c, title).FontSize(14).Bold()
		if len(p.courseOpts) > 0 {
			design.SelectField(c, "课程", &p.selectedCourse, p.courseOpts)
		}
		if len(p.teacherOpts) > 0 {
			design.SelectField(c, "任课老师", &p.selectedTeacher, p.teacherOpts)
		}
		if len(p.roomOpts) > 0 {
			design.SelectField(c, "安排教室", &p.selectedRoom, p.roomOpts)
		}
		design.TextField(c, "上课时间", &p.startAtStr, "格式: 2006-01-02 15:04")
		design.IntField(c, "课程时长 (分钟)", &p.draft.Minutes, 15, 360)
		design.SelectField(c, "排课状态", &p.selectedStatus, p.statusOpts)
		design.FormError(c, p.formErr)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if !p.isNew && ui.Button(c, "删除").Clicked() {
				p.askingDelete = true
			}
			ui.Spacer(c)
			design.FormActions(c, func() { p.editing = false }, p.save, "保存")
		})
	}).Width(design.ToolbarWidth).Shrink(0)
}

func (p *Lessons) confirmDelete(c *ui.Context) {
	if !design.Confirm(c, &p.askingDelete, "删除该排课？", "删除排课记录后将从时间表移除，操作不可撤销。", "删除") {
		return
	}
	if p.draft.ID != "" {
		p.remove(p.draft.ID)
	}
}

func nameOr(name, fallback string) string {
	if name == "" {
		return fallback
	}
	return name
}

func lessonStatus(status string) (string, design.Tone) {
	switch status {
	case "done":
		return "已完成", design.ToneNeutral
	case "canceled":
		return "已取消", design.ToneDanger
	default:
		return "待上课", design.ToneAccent
	}
}
