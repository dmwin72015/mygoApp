package features

import (
	"context"
	"fmt"
	"sort"

	"mygo-app/internal/design"
	"mygo-app/internal/domain"
	"mygo-app/internal/i18n"

	"github.com/egoist/mygo/ui"
)

// Dashboard is 概览: the numbers of the school, the lessons coming up and
// the money that moved last. It reads four sources at once on one
// goroutine, so the page takes one round trip, not four frames.
type Dashboard struct {
	repo domain.Repo
	win  design.Updater

	stats    domain.Stats
	lessons  []domain.Lesson
	entries  []domain.LedgerEntry
	students map[string]string
	courses  map[string]string

	loaded, busy bool
	err          error
}

// NewDashboard builds the 概览 page.
func NewDashboard(repo domain.Repo, win design.Updater) *Dashboard {
	return &Dashboard{repo: repo, win: win}
}

func (d *Dashboard) reload() {
	d.busy = true
	win, repo := d.win, d.repo
	go func() {
		ctx := context.Background()
		stats, err := repo.Stats(ctx)
		lessons, _ := repo.Lessons().List(ctx, domain.Query{})
		entries, _ := repo.Ledger().List(ctx, "", 8)
		students, _ := repo.Students().List(ctx, domain.Query{})
		courses, _ := repo.Courses().List(ctx, domain.Query{})
		win.Update(func() {
			d.stats, d.lessons, d.entries = stats, lessons, entries
			d.students = make(map[string]string, len(students))
			for _, v := range students {
				d.students[v.ID] = v.Name
			}
			d.courses = make(map[string]string, len(courses))
			for _, v := range courses {
				d.courses[v.ID] = v.Name
			}
			d.err, d.busy, d.loaded = err, false, true
		})
	}()
}

func (d *Dashboard) View(c *ui.Context) {
	if !d.loaded && !d.busy {
		d.reload()
	}
	t := design.Theme(c)
	design.Page(c, design.PageProps{
		Title:    "概览",
		Subtitle: "今天该做什么，一眼看到",
		Scroll:   true,
		Actions: func() {
			if ui.Button(c, "刷新").Clicked() {
				d.reload()
			}
		},
	}, func() {
		design.ErrorBar(c, d.err)

		ui.Row(c).Gap(design.Gap).AlignItems(ui.Stretch).Children(func() {
			stat := func(label, value, hint string) {
				ui.Box(c).Grow(1).MinWidth(0).Children(func() { design.Stat(c, label, value, hint) })
			}
			stat("在读学生", fmt.Sprint(d.stats.Students), "名学员")
			stat("在职老师", fmt.Sprint(d.stats.Teachers), "位老师")
			stat("在售课程", fmt.Sprint(d.stats.Courses), "门课程")
			stat("待上课程", fmt.Sprint(d.stats.Lessons), "节排课")
		})
		ui.Row(c).Gap(design.Gap).AlignItems(ui.Stretch).Children(func() {
			stat := func(label, value, hint string) {
				ui.Box(c).Grow(1).MinWidth(0).Children(func() { design.Stat(c, label, value, hint) })
			}
			stat("累计充值", design.Money(d.stats.Revenue), "全部充值流水")
			stat("预存余额", design.Money(d.stats.Outstanding), "尚未消课的余额")
		})

		design.SectionCard(c, "近期排课", nil, func() {
			upcoming := d.upcoming(5)
			if len(upcoming) == 0 {
				ui.Text(c, "暂无待上的课").FontSize(12).TextColor(t.TextMuted)
				return
			}
			for _, v := range upcoming {
				v := v
				ui.Row(c).Gap(10).AlignItems(ui.Center).PaddingY(5).Children(func() {
					ui.Text(c, design.DateTime(v.StartAt)).FontSize(12).Width(130).TextColor(t.TextMuted)
					ui.Text(c, nameOr(d.courses[v.CourseID], "未命名课程")).FontSize(13).Grow(1).SingleLine()
					ui.Textf(c, "%d 分钟", v.Minutes).FontSize(12).TextColor(t.TextMuted)
				})
			}
		})

		design.SectionCard(c, "最近流水", nil, func() {
			if len(d.entries) == 0 {
				ui.Text(c, "还没有充值或消课记录").FontSize(12).TextColor(t.TextMuted)
				return
			}
			for _, e := range d.entries {
				e := e
				label, tone := "充值", design.TonePositive
				if e.Kind == domain.KindConsume {
					label, tone = "消课", design.ToneWarning
				}
				ui.Row(c).Gap(10).AlignItems(ui.Center).PaddingY(5).Children(func() {
					design.Pill(c, label, tone)
					ui.Text(c, nameOr(d.students[e.StudentID], "—")).FontSize(13).Width(110).SingleLine()
					ui.Text(c, e.Note).FontSize(12).TextColor(t.TextMuted).Grow(1).SingleLine()
					ui.Text(c, design.Money(e.Amount)).FontSize(12).Bold()
				})
			}
		})
	})
}

func (d *Dashboard) upcoming(n int) []domain.Lesson {
	out := make([]domain.Lesson, 0, len(d.lessons))
	for _, v := range d.lessons {
		if v.Status == "planned" {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartAt.Before(out[j].StartAt) })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// Settings is 系统设置: what the app reads and writes, and how to point it
// somewhere else. It is the page to grow with the app's own preferences.
type Settings struct {
	repo domain.Repo
	win  design.Updater
	self domain.User

	oldPassword string
	newPassword string
	confirmPwd  string
	pwdBusy     bool
	pwdErr      string
	pwdSuccess  string
}

// NewSettings builds the 系统设置 page.
func NewSettings(repo domain.Repo, win design.Updater, self domain.User) *Settings {
	return &Settings{repo: repo, win: win, self: self}
}

func (s *Settings) changePassword() {
	if s.oldPassword == "" {
		s.pwdErr = i18n.T("settings.pwd_err_old")
		s.pwdSuccess = ""
		return
	}
	if len([]rune(s.newPassword)) < 6 {
		s.pwdErr = i18n.T("settings.pwd_err_len")
		s.pwdSuccess = ""
		return
	}
	if s.newPassword != s.confirmPwd {
		s.pwdErr = i18n.T("settings.pwd_err_match")
		s.pwdSuccess = ""
		return
	}
	s.pwdBusy, s.pwdErr, s.pwdSuccess = true, "", ""
	repo, win, self, old, newPwd := s.repo, s.win, s.self, s.oldPassword, s.newPassword
	go func() {
		ctx := context.Background()
		_, err := repo.Auth().Login(ctx, self.Username, old)
		if err != nil {
			win.Update(func() {
				s.pwdBusy = false
				s.pwdErr = i18n.T("settings.pwd_err_wrong")
			})
			return
		}
		err = repo.Users().SetPassword(ctx, self.ID, newPwd)
		win.Update(func() {
			s.pwdBusy = false
			if err != nil {
				s.pwdErr = err.Error()
				return
			}
			s.oldPassword, s.newPassword, s.confirmPwd = "", "", ""
			s.pwdSuccess = i18n.T("settings.pwd_success")
		})
	}()
}

func (s *Settings) View(c *ui.Context) {
	t := design.Theme(c)
	source := "未连接"
	if s.repo != nil {
		source = s.repo.Source()
	}
	design.Page(c, design.PageProps{
		Title:    i18n.T("settings.title"),
		Subtitle: i18n.T("settings.subtitle"),
		Scroll:   true,
	}, func() {
		design.SectionCard(c, i18n.T("settings.lang_card"), nil, func() {
			design.LanguageSelector(c)
			ui.Text(c, i18n.T("settings.lang_desc")).
				FontSize(12).TextColor(t.TextMuted)
		})

		if s.self.ID != "" {
			design.SectionCard(c, i18n.T("settings.pwd_card"), nil, func() {
				ui.Field(c, i18n.T("settings.pwd_old"), func() {
					ui.TextInput(c, &s.oldPassword).Password().Placeholder(i18n.T("settings.pwd_old_ph"))
				})
				ui.Field(c, i18n.T("settings.pwd_new"), func() {
					ui.TextInput(c, &s.newPassword).Password().Placeholder(i18n.T("settings.pwd_new_ph"))
				})
				ui.Field(c, i18n.T("settings.pwd_confirm"), func() {
					ui.TextInput(c, &s.confirmPwd).Password().Placeholder(i18n.T("settings.pwd_conf_ph"))
				})
				design.FormError(c, s.pwdErr)
				if s.pwdSuccess != "" {
					ui.Text(c, s.pwdSuccess).FontSize(12).TextColor(t.Success)
				}
				ui.Row(c).Justify(ui.End).Children(func() {
					if ui.PrimaryButton(c, i18n.T("settings.pwd_btn")).Disabled(s.pwdBusy).Clicked() {
						s.changePassword()
					}
				})
			})
		}
		design.SectionCard(c, i18n.T("settings.datasource"), nil, func() {
			kv(c, i18n.T("settings.source_curr"), source)
			kv(c, i18n.T("settings.source_sqlite"), "SQLite")
			kv(c, i18n.T("settings.source_remote"), "REST API")
		})
		design.SectionCard(c, i18n.T("settings.appearance"), nil, func() {
			design.Appearance(c)
			ui.Text(c, i18n.T("settings.theme_desc")).
				FontSize(12).TextColor(t.TextMuted)
			kv(c, i18n.T("settings.accent"), i18n.T("settings.accent_val"))
		})
	})
}

// kv is a labelled line of a settings card.
func kv(c *ui.Context, k, v string) {
	t := design.Theme(c)
	ui.Row(c).Gap(10).AlignItems(ui.Center).PaddingY(3).Children(func() {
		ui.Text(c, k).FontSize(12).TextColor(t.TextMuted).Width(90)
		ui.Text(c, v).FontSize(12).Grow(1).SingleLine()
	})
}
