package components

import (
	"mygo-app/internal/design/tokens"

	"github.com/egoist/mygo/ui"
)

// ModernCard wraps children inside a clean, solid, cross-platform neutral card container.
func ModernCard(c *ui.Context, fn func()) ui.Element {
	tok := tokens.Get(c)
	return ui.Column(c).
		Padding(tokens.SpaceLg).
		Gap(tokens.SpaceMd).
		Radius(tokens.RadiusLg).
		Background(tok.CardSurface).
		Border(tokens.BorderThin, tok.BorderDefault).
		Shadow(0, 1, 2, 0, tok.ShadowColor).
		Children(fn)
}

// DividedGroup groups rows into a single card and automatically draws a 1px hairline divider between children.
func DividedGroup(c *ui.Context, fn func()) ui.Element {
	tok := tokens.Get(c)
	return ui.Column(c).
		Radius(tokens.RadiusLg).
		Background(tok.CardSurface).
		Border(tokens.BorderThin, tok.BorderDefault).
		Dividers(tokens.BorderThin, tok.BorderMuted).
		Children(fn)
}

// StatusTone defines the color theme of a StatusBadge.
type StatusTone int

const (
	ToneNeutral StatusTone = iota
	ToneSuccess
	ToneWarning
	ToneDanger
	ToneBrand
)

// StatusBadge renders a compact, elegant badge with balanced tint background.
func StatusBadge(c *ui.Context, label string, tone StatusTone) ui.Element {
	tok := tokens.Get(c)
	box := ui.Row(c).
		Padding(2, 8).
		Radius(tokens.RadiusSm).
		AlignItems(ui.Center).
		Justify(ui.Center)

	var bg, text ui.Color
	switch tone {
	case ToneSuccess:
		bg = tok.Success.Alpha(0.12)
		text = tok.Success
	case ToneWarning:
		bg = tok.Warning.Alpha(0.14)
		text = tok.Warning
	case ToneDanger:
		bg = tok.Danger.Alpha(0.12)
		text = tok.Danger
	case ToneBrand:
		bg = tok.BrandPrimary.Alpha(0.12)
		text = tok.BrandPrimary
	default:
		bg = tok.PageBg
		text = tok.TextSecondary
		box.Border(tokens.BorderThin, tok.BorderDefault)
	}

	box.Background(bg)
	box.Children(func() {
		ui.Text(c, label).
			FontSize(11.5).
			FontWeight(600).
			TextColor(text).
			SingleLine()
	})

	return box
}

// ModernTable wraps ui.Table with scoped modern neutral selection colors,
// ensuring that selected rows have a soft, comfortable background tint (instead of dark solid accent)
// and all text, badges, and numbers remain crisp and high contrast.
func ModernTable(c *ui.Context, s *ui.ListState, columns []ui.TableColumn, n int, cell func(row, col int)) ui.Element {
	tok := tokens.Get(c)
	th := *c.Theme()
	th.Accent = tok.Selection
	th.AccentText = tok.TextPrimary
	if !th.Dark {
		th.SurfaceHover = tok.BrandPrimary.Alpha(0.04)
	} else {
		th.SurfaceHover = tok.BrandPrimary.Alpha(0.08)
	}

	var tableElem ui.Element
	ui.Themed(c, &th).Grow(1).Background(ui.Transparent).Children(func() {
		tableElem = ui.Table(c, s, columns, n, cell).Grow(1)
	})
	return tableElem
}

