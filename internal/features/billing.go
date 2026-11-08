package features

import (
	"context"
	"fmt"
	"strings"

	"mygo-app/internal/design"
	"mygo-app/internal/domain"

	"github.com/egoist/mygo/ui"
)

// Billing is 充值 / 消课: the students on the left, the operation and the
// account history of the chosen one on the right. Every cent moves through
// the ledger, so the balance always has a history.
type Billing struct {
	repo domain.Repo
	win  design.Updater
	// canWrite is false for a role that only reads: the page keeps the
	// balance and the history, and hides the form that moves money.
	canWrite bool

	students []domain.Student
	entries  []domain.LedgerEntry

	loaded, busy bool
	err          error

	tbl   ui.ListState
	sel   int
	selID int64

	consume bool
	// amountCents is what the money field edits, in cents.
	amountCents int64
	note        string
	msg         string
	msgErr      bool
}

// NewBilling builds the 充值 / 消课 page.
func NewBilling(repo domain.Repo, win design.Updater, canWrite bool) *Billing {
	return &Billing{repo: repo, win: win, canWrite: canWrite, sel: -1}
}

func (b *Billing) current() (domain.Student, bool) {
	if b.sel < 0 || b.sel >= len(b.students) {
		return domain.Student{}, false
	}
	return b.students[b.sel], true
}

func (b *Billing) reload() {
	b.busy = true
	id := b.selID
	win, repo := b.win, b.repo
	go func() {
		ctx := context.Background()
		students, err := repo.Students().List(ctx, domain.Query{})
		var entries []domain.LedgerEntry
		if err == nil {
			if id == 0 && len(students) > 0 {
				id = students[0].ID
			}
			if id != 0 {
				entries, _ = repo.Ledger().List(ctx, id, 50)
			}
		}
		win.Update(func() {
			b.students, b.entries = students, entries
			b.err, b.busy, b.loaded = err, false, true
			if b.sel >= len(b.students) {
				b.sel = len(b.students) - 1
			}
			b.selID = id
		})
	}()
}

func (b *Billing) reloadEntries() {
	id := b.selID
	if id == 0 {
		return
	}
	win, repo := b.win, b.repo
	go func() {
		entries, err := repo.Ledger().List(context.Background(), id, 50)
		win.Update(func() {
			b.entries = entries
			if err != nil {
				b.err = err
			}
		})
	}()
}

func (b *Billing) submit() {
	v, ok := b.current()
	if !ok {
		b.msg, b.msgErr = "先在左侧选择一名学生", true
		return
	}
	if b.amountCents <= 0 {
		b.msg, b.msgErr = "金额必须大于 0", true
		return
	}
	amount := b.amountCents
	note := strings.TrimSpace(b.note)
	consume := b.consume
	b.busy = true
	b.msg, b.msgErr = "", false
	win, repo := b.win, b.repo
	go func() {
		ctx := context.Background()
		var err error
		if consume {
			_, err = repo.Ledger().Consume(ctx, v.ID, amount, note)
		} else {
			_, err = repo.Ledger().TopUp(ctx, v.ID, amount, note)
		}
		win.Update(func() {
			b.busy = false
			if err != nil {
				b.msg, b.msgErr = err.Error(), true
				return
			}
			if consume {
				b.msg = fmt.Sprintf("已为 %s 消课 %s", v.Name, design.Money(amount))
			} else {
				b.msg = fmt.Sprintf("已为 %s 充值 %s", v.Name, design.Money(amount))
			}
			b.amountCents, b.note = 0, ""
			b.reload()
		})
	}()
}

func (b *Billing) View(c *ui.Context) {
	if !b.loaded && !b.busy {
		b.reload()
	}
	// Choosing another row loads that account's history.
	if v, ok := b.current(); ok && v.ID != b.selID {
		b.selID = v.ID
		b.reloadEntries()
	}
	design.Page(c, design.PageProps{
		Title:    "充值 / 消课",
		Subtitle: "预存学费与按节消课，每次变动都记一笔流水",
		Actions: func() {
			if ui.Button(c, "刷新").Clicked() {
				b.reload()
			}
		},
	}, func() {
		design.ErrorBar(c, b.err)
		ui.Row(c).Fill().Grow(1).MinHeight(0).Gap(design.Gap).AlignItems(ui.Stretch).Children(func() {
			b.studentPane(c)
			ui.Column(c).Width(design.ToolbarWidth).Shrink(0).Gap(design.Gap).Children(func() {
				if b.canWrite {
					b.actionPane(c)
				} else {
					design.Card(c, func() {
						t := design.Theme(c)
						ui.Text(c, "只读账号").FontSize(13).Bold()
						ui.Text(c, "你可以查看余额与流水，充值、消课需要教务或管理员权限。").
							FontSize(12).TextColor(t.TextMuted)
					})
				}
				b.historyPane(c)
			})
		})
	})
}

func (b *Billing) studentPane(c *ui.Context) {
	t := design.Theme(c)
	design.Card(c, func() {
		ui.Text(c, "学生账户").FontSize(13).Bold()
		if len(b.students) == 0 {
			if b.busy {
				design.Loading(c, "学生")
			} else {
				design.Empty(c, "还没有学生", "先到「学生管理」建档。")
			}
			return
		}
		cols := []ui.TableColumn{
			{Title: "姓名", Width: 130},
			{Title: "家长", Width: 110},
			{Title: "余额", Width: 120, Align: ui.End},
			{Title: "状态", Width: 80},
		}
		b.tbl.Key = func(row int) any { return b.students[row].ID }
		b.tbl.Selected = &b.sel
		design.Table(c, &b.tbl, cols, len(b.students), func(row, col int) {
			v := b.students[row]
			switch col {
			case 0:
				ui.Text(c, v.Name).SingleLine()
			case 1:
				ui.Text(c, v.Guardian).SingleLine()
			case 2:
				ui.Text(c, design.Money(v.Balance)).SingleLine()
			case 3:
				design.Pill(c, activeLabel(v.Active), activeTone(v.Active))
			}
		}).Grow(1)
		ui.Text(c, "选一名学生，右侧充值或消课").FontSize(11).TextColor(t.TextMuted)
	}).Grow(1).MinWidth(0)
}

func (b *Billing) actionPane(c *ui.Context) {
	t := design.Theme(c)
	design.Card(c, func() {
		v, ok := b.current()
		if !ok {
			ui.Text(c, "未选择学生").FontSize(13).TextColor(t.TextMuted)
			return
		}
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, v.Name).FontSize(13).Bold()
			ui.Spacer(c)
			ui.Text(c, design.Money(v.Balance)).FontSize(18).Bold().TextColor(t.Accent)
		})
		kind := 0
		if b.consume {
			kind = 1
		}
		if ui.Segmented(c, &kind, "充值", "消课").Changed() {
			b.consume = kind == 1
			b.msg = ""
		}
		design.MoneyField(c, "金额（元）", &b.amountCents)
		design.TextField(c, "备注", &b.note, "如：报名初二数学同步")
		if b.msg != "" {
			col := t.Success
			if b.msgErr {
				col = t.Danger
			}
			ui.Text(c, b.msg).FontSize(12).TextColor(col)
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			label := "确认充值"
			if b.consume {
				label = "确认消课"
			}
			if ui.PrimaryButton(c, label).Clicked() {
				b.submit()
			}
		})
	})
}

func (b *Billing) historyPane(c *ui.Context) {
	t := design.Theme(c)
	design.Card(c, func() {
		ui.Text(c, "最近流水").FontSize(13).Bold()
		if len(b.entries) == 0 {
			ui.Text(c, "暂无记录").FontSize(12).TextColor(t.TextMuted)
			return
		}
		ui.Scroll(c).Grow(1).Gap(6).Children(func() {
			for _, e := range b.entries {
				e := e
				ui.Row(c).Gap(8).AlignItems(ui.Center).PaddingY(4).Children(func() {
					label, tone := "充值", design.TonePositive
					if e.Kind == domain.KindConsume {
						label, tone = "消课", design.ToneWarning
					}
					design.Pill(c, label, tone)
					ui.Column(c).Grow(1).Gap(1).MinWidth(0).Children(func() {
						ui.Text(c, design.DateTime(e.At)).FontSize(12).SingleLine()
						if e.Note != "" {
							ui.Text(c, e.Note).FontSize(11).TextColor(t.TextMuted).SingleLine()
						}
					})
					ui.Column(c).Gap(1).AlignItems(ui.End).Children(func() {
						ui.Text(c, design.Money(e.Amount)).FontSize(12).Bold()
						ui.Text(c, design.Money(e.Balance)).FontSize(11).TextColor(t.TextMuted)
					})
				})
			}
		})
	}).Grow(1)
}
