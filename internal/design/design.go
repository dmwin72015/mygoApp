// Package design is the app's design system: the theme, the frame every
// page sits in, and the small pieces (cards, nav rows, pills, empty states)
// that keep the screens looking like one product. It knows nothing about
// students or lessons, so it is reused as the app grows.
package design

import (
	"fmt"
	"strings"
	"time"

	"mygo-app/internal/design/components"
	"mygo-app/internal/design/tokens"

	"github.com/egoist/mygo/ui"
)

// Metrics, in DIPs.
const (
	PagePadding = tokens.SpaceLg
	CardPadding = tokens.SpaceLg
	Gap         = tokens.SpaceMd
	Radius      = tokens.RadiusLg
	SideWidth   = tokens.SidebarWidth
	// ToolbarWidth is the width of the editor panel beside a list.
	ToolbarWidth = float32(360)
)

// PageProps describes the frame of a screen.
type PageProps struct {
	Title    string
	Subtitle string
	// Actions builds the buttons at the right of the header.
	Actions func()
	// Scroll wraps the body in a scroll container. Leave it false for a
	// page whose table fills the room and scrolls on its own.
	Scroll bool
}

// Page is the frame of every screen: the page color behind, a header with
// the title and the actions, then the body.
func Page(c *ui.Context, p PageProps, body func()) {
	t, look := Theme(c), LookFor(c)
	ui.Column(c).Fill().Background(look.Page).Children(func() {
		ui.Row(c).Padding(PagePadding+4, PagePadding+4).Gap(Gap).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Shrink(0).Children(func() {
			ui.Column(c).Grow(1).Gap(3).MinWidth(0).Children(func() {
				ui.Text(c, p.Title).FontSize(19).Bold()
				if p.Subtitle != "" {
					ui.Text(c, p.Subtitle).FontSize(12).TextColor(t.TextMuted).SingleLine()
				}
			})
			if p.Actions != nil {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(p.Actions)
			}
		})
		if p.Scroll {
			ui.Scroll(c).Grow(1).Padding(PagePadding+4, PagePadding+4).Gap(Gap).Children(body)
			return
		}
		ui.Column(c).Grow(1).Fill().MinHeight(0).Padding(PagePadding+4, PagePadding+4).
			Gap(Gap).Children(body)
	})
}

// Card groups related content on a surface, a step above the page.
func Card(c *ui.Context, fn func()) ui.Element {
	t, look := Theme(c), LookFor(c)
	return ui.Column(c).Padding(CardPadding).Gap(Gap).Radius(Radius).
		Background(look.Card).BorderWidth(1).BorderColor(t.Border).
		Shadow(0, 1, 2, 0, look.Shadow).Children(fn)
}

// SectionCard is a Card with a title row, and an optional action on the
// right, as a table with a "查看全部" link.
func SectionCard(c *ui.Context, title string, action func(), fn func()) {
	Card(c, func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, title).FontSize(13).Bold().Grow(1)
			if action != nil {
				action()
			}
		})
		fn()
	})
}

// Stat is one number of the overview: what it counts, the number, and a
// hint under it. The card lifts a little under the pointer. A long value,
// as a sum of money, steps down in size so it stays on one line.
func Stat(c *ui.Context, label, value, hint string) {
	t := Theme(c)
	card := Card(c, func() {
		ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
		size := float32(26)
		switch n := len([]rune(value)); {
		case n > 9:
			size = 20
		case n > 6:
			size = 23
		}
		ui.Text(c, value).FontSize(size).Bold().SingleLine()
		if hint != "" {
			ui.Text(c, hint).FontSize(11).TextColor(t.TextMuted).SingleLine()
		}
	})
	lift := float32(0)
	if card.Hovered() {
		lift = -3
	}
	card.Translate(0, card.AnimateTo("lift", lift, ui.Snappy))
}

// NavRow is one row of the sidebar: the chosen one carries a tint of the
// accent, the others a hover that fades in. It reports a click.
func NavRow(c *ui.Context, label string, active bool) bool {
	t, look := Theme(c), LookFor(c)
	row := ui.Row(c).Height(32).Radius(8).Padding(0, 10).AlignItems(ui.Center).Focusable()
	row.Cursor(ui.CursorPointer)
	target := ui.Color{}
	text := t.TextMuted
	switch {
	case active:
		target, text = look.NavActive, t.Accent
	case row.Hovered():
		target = t.SurfaceHover
	}
	row.Background(row.AnimateColor("nav-bg", target, ui.Snappy))
	row.Children(func() {
		label := ui.Text(c, label).FontSize(13).TextColor(row.AnimateColor("nav-fg", text, ui.Snappy))
		if active {
			label.Bold()
		}
	})
	return row.Clicked()
}

// Tone is the meaning of a pill, which picks its color.
type Tone int

// Tones a pill can take.
const (
	ToneNeutral Tone = iota
	TonePositive
	ToneWarning
	ToneDanger
	ToneAccent
)

// Pill is a small status label, as 在读 or 已消课.
func Pill(c *ui.Context, text string, tone Tone) {
	dark := Theme(c).Dark
	var fg, bg ui.Color
	if !dark {
		switch tone {
		case TonePositive:
			fg, bg = ui.Hex("#15803d"), ui.RGBA(22, 163, 74, 0.12)
		case ToneWarning:
			fg, bg = ui.Hex("#b45309"), ui.RGBA(217, 119, 6, 0.12)
		case ToneDanger:
			fg, bg = ui.Hex("#b91c1c"), ui.RGBA(220, 38, 38, 0.12)
		case ToneAccent:
			fg, bg = ui.Hex("#4338ca"), ui.RGBA(79, 70, 229, 0.12)
		default:
			fg, bg = ui.Hex("#667085"), ui.Hex("#f2f4f7")
		}
	} else {
		switch tone {
		case TonePositive:
			fg, bg = ui.Hex("#4ade80"), ui.RGBA(74, 222, 128, 0.14)
		case ToneWarning:
			fg, bg = ui.Hex("#fbbf24"), ui.RGBA(251, 191, 36, 0.14)
		case ToneDanger:
			fg, bg = ui.Hex("#f87171"), ui.RGBA(248, 113, 113, 0.14)
		case ToneAccent:
			fg, bg = ui.Hex("#a5b4fc"), ui.RGBA(129, 140, 248, 0.16)
		default:
			fg, bg = ui.Hex("#949bab"), ui.RGBA(148, 155, 171, 0.14)
		}
	}
	ui.Row(c).Padding(2, 8).Radius(999).Background(bg).Shrink(0).Children(func() {
		ui.Text(c, text).FontSize(11).TextColor(fg).SingleLine()
	})
}

// Empty is what a list shows before it has anything to show.
func Empty(c *ui.Context, title, hint string) {
	t := Theme(c)
	ui.Column(c).Fill().Center().Gap(6).Padding(40).Children(func() {
		ui.Text(c, title).FontSize(14).Bold().TextColor(t.TextMuted)
		if hint != "" {
			ui.Text(c, hint).FontSize(12).TextColor(t.TextMuted).TextAlign(ui.Center)
		}
	})
}

// Loading is what a panel shows while its data is on its way.
func Loading(c *ui.Context, what string) {
	t := Theme(c)
	ui.Row(c).Fill().Center().Gap(8).Padding(40).Children(func() {
		ui.Spinner(c).Width(16)
		ui.Text(c, "正在加载"+what+"…").FontSize(12).TextColor(t.TextMuted)
	})
}

// ErrorBar shows a failed operation without taking the page away.
func ErrorBar(c *ui.Context, err error) {
	if err == nil {
		return
	}
	t := Theme(c)
	ui.Row(c).Padding(10, 12).Radius(10).Background(ui.RGBA(220, 38, 38, 0.08)).
		BorderWidth(1).BorderColor(ui.RGBA(220, 38, 38, 0.25)).Gap(8).Children(func() {
		ui.Text(c, err.Error()).FontSize(12).TextColor(t.Danger).Grow(1)
	})
}

// Money formats cents as 元, with a sign and thousands separators.
func Money(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	whole := fmt.Sprintf("%d", cents/100)
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("%s¥%s.%02d", sign, b.String(), cents%100)
}

// Yuan turns cents into the number a field edits.
func Yuan(cents int64) float64 { return float64(cents) / 100 }

// Cents turns what a field holds back into cents.
func Cents(yuan float64) int64 { return int64(yuan*100 + 0.5) }

// Date formats a day.
func Date(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("2006-01-02")
}

// DateTime formats a moment.
func DateTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("2006-01-02 15:04")
}

// Updater schedules work back onto the UI thread (like *mygo.Window.Update),
// or executes it synchronously in headless tests.
type Updater interface {
	Update(func())
}

// SyncUpdater executes updates immediately, allowing features to be tested
// without a physical window.
type SyncUpdater struct{}

func (SyncUpdater) Update(fn func()) {
	if fn != nil {
		fn()
	}
}

// Facade re-exports for modern components
type ButtonVariant = components.ButtonVariant

const (
	ButtonPrimary   = components.ButtonPrimary
	ButtonSecondary = components.ButtonSecondary
	ButtonDanger    = components.ButtonDanger
	ButtonGhost     = components.ButtonGhost
)

var (
	ActionButton    = components.ActionButton
	IconButton      = components.IconButton
	SegmentedPicker = components.SegmentedPicker
	SmoothSwitch    = components.SmoothSwitch
	SearchField     = components.SearchField
	ModernCard      = components.ModernCard
	DividedGroup    = components.DividedGroup
	StatusBadge     = components.StatusBadge
	ModernTable     = components.ModernTable
	Heading         = components.Heading
	NumText         = components.NumText
)

// Table wraps ui.Table with modern soft row-selection styling and high text contrast.
func Table(c *ui.Context, s *ui.ListState, columns []ui.TableColumn, n int, cell func(row, col int)) ui.Element {
	return components.ModernTable(c, s, columns, n, cell)
}
