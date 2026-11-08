package components

import (
	"time"

	"mygo-app/internal/design/tokens"

	"github.com/egoist/mygo/ui"
)

// ButtonVariant determines the visual hierarchy of an ActionButton.
type ButtonVariant int

const (
	ButtonPrimary ButtonVariant = iota
	ButtonSecondary
	ButtonDanger
	ButtonGhost
)

// ActionButton is a cross-platform modern neutral button built on ui.ButtonBase.
// It features smooth 120ms color transitions on hover and a tactile 0.97 press scale.
func ActionButton(c *ui.Context, label string, variant ButtonVariant) ui.Element {
	tok := tokens.Get(c)
	b := ui.ButtonBase(c).
		Height(tokens.ControlHeight).
		PaddingX(14).
		Radius(tokens.RadiusMd).
		AlignItems(ui.Center).
		Justify(ui.Center).
		FocusRing(true)

	var bg, hoverBg, textColor ui.Color
	switch variant {
	case ButtonPrimary:
		bg = tok.BrandPrimary
		hoverBg = tok.BrandHover
		textColor = tok.TextOnAccent
	case ButtonDanger:
		bg = tok.Danger
		hoverBg = tok.Danger.Mix(ui.Hex("#000000"), 0.12)
		textColor = tok.TextOnAccent
	case ButtonGhost:
		bg = ui.Transparent
		hoverBg = tok.Selection
		textColor = tok.TextPrimary
	default: // ButtonSecondary
		bg = tok.CardSurface
		hoverBg = tok.CardSurface.Mix(tok.BorderDefault, 0.25)
		textColor = tok.TextPrimary
		b.Border(tokens.BorderThin, tok.BorderDefault)
	}

	fill := bg
	if b.Hovered() {
		fill = hoverBg
	}
	b.Background(b.AnimateColor("btn_bg", fill, ui.Animation{Duration: 120 * time.Millisecond}))

	pose := ui.Pose{}
	if b.Pressed() {
		pose.Scale = 0.97
	}
	b.AnimatePose(pose, ui.Snappy)

	b.Children(func() {
		ui.Text(c, label).
			FontSize(13).
			FontWeight(600).
			TextColor(textColor).
			SingleLine()
	})

	return b
}

// IconButton is a compact square or rounded icon button with micro-interactions.
func IconButton(c *ui.Context, icon *ui.SVG, size float32, label string) ui.Element {
	tok := tokens.Get(c)
	btnSize := size + 16
	b := ui.ButtonBase(c).
		Size(btnSize, btnSize).
		Center().
		Radius(tokens.RadiusMd).
		Label(label).
		TextColor(tok.TextPrimary)

	bg := ui.Transparent
	if b.Hovered() {
		bg = tok.Selection
	}
	b.Background(b.AnimateColor("icon_btn_bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))

	pose := ui.Pose{}
	if b.Pressed() {
		pose.Scale = 0.94
	}
	b.AnimatePose(pose, ui.Snappy)

	b.Children(func() {
		ui.Icon(c, icon).Size(size, size)
	})

	return b
}
