package components

import (
	"time"

	"mygo-app/internal/design/tokens"

	"github.com/egoist/mygo/ui"
)

// SegmentedPicker is an Apple/Fluent-grade segmented control with an animated
// raised capsule, dual soft shadows, and clean hairline dividers.
// It reports true if the user changed the selection.
func SegmentedPicker(c *ui.Context, key string, selected *int, options []string) bool {
	if len(options) == 0 {
		return false
	}
	tok := tokens.Get(c)
	const height = float32(28)
	const inset = float32(2)
	radius := tokens.RadiusSm

	// Dynamically compute width to ensure all segments are uniformly and comfortably sized
	width := float32(0)
	for _, opt := range options {
		w, _ := c.MeasureText(0, ui.Span{Text: opt, Size: 13})
		if w+24 > width {
			width = w + 24
		}
	}

	trackBg := tok.PageBg
	capsuleBg := tok.CardSurface
	dividerColor := tok.BorderMuted

	before := *selected
	parts := ui.SegmentedBase(c.Key(key), selected, len(options))

	// Spring animation for the sliding capsule (280ms with 0.08 bounce)
	at := parts.Track.AnimateTo("seg_at", float32(*selected), ui.Animation{
		Spring: ui.SpringOptions{Duration: 280 * time.Millisecond, Bounce: 0.08},
	})

	n := len(options)
	dark := tok.Dark

	parts.Track.
		Height(height).
		Padding(inset).
		Radius(radius + 2).
		Background(trackBg).
		Border(tokens.BorderThin, tok.BorderDefault).
		Draw(func(p *ui.Painter, r ui.Rect) {
			// Hairline dividers between segments, skipping the one under or adjacent to the capsule
			for i := 1; i < n; i++ {
				if d := float32(i) - at; d > -0.8 && d < 1.8 {
					continue
				}
				x := r.X + inset + float32(i)*width
				p.Fill(ui.Rect{X: x - 0.5, Y: r.Y + 4, W: 1, H: r.H - 8}, dividerColor, 0)
			}

			// The sliding pill
			pill := ui.Rect{
				X: r.X + inset + at*width,
				Y: r.Y + inset,
				W: width,
				H: r.H - 2*inset,
			}
			if !dark {
				p.Shadow(pill, radius, 0, 0.5, 1, 0, ui.RGBA(0, 0, 0, 0.12))
				p.Shadow(pill, radius, 0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.04))
			}
			p.Fill(pill, capsuleBg, radius)
		}).
		Children(func() {
			for i, opt := range options {
				seg := parts.Segment(i).
					Width(width).
					Height(height - 2*inset).
					Center().
					Radius(radius)

				if seg.Pressed() && i != *selected {
					seg.Background(dividerColor)
				}

				seg.Children(func() {
					txt := ui.Text(c, opt).FontSize(12.5).SingleLine()
					if i == *selected {
						txt.FontWeight(600).TextColor(tok.TextPrimary)
					} else {
						txt.TextColor(tok.TextSecondary)
					}
				})
			}
		})

	return *selected != before
}

// SmoothSwitch is a smooth toggling switch with animated sliding knob.
func SmoothSwitch(c *ui.Context, on *bool) ui.Element {
	tok := tokens.Get(c)
	sw := ui.SwitchBase(c, on).
		Size(42, 24).
		Radius(12).
		Padding(2).
		AlignItems(ui.Center)

	// Animate background color transition
	targetBg := tok.BorderDefault
	if *on {
		targetBg = tok.BrandPrimary
	}
	sw.Background(sw.AnimateColor("sw_bg", targetBg, ui.Animation{Duration: 160 * time.Millisecond}))

	// Animate knob translation
	targetX := float32(0)
	if *on {
		targetX = 18
	}
	knobX := sw.AnimateTo("sw_knob", targetX, ui.Animation{Duration: 160 * time.Millisecond})

	sw.Children(func() {
		ui.Box(c).
			Size(20, 20).
			Radius(10).
			Background(ui.Hex("#ffffff")).
			Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.20)).
			Translate(knobX, 0)
	})

	return sw
}

// SearchField is a modern search input featuring a Tailwind/Linear-style focus ring glow.
func SearchField(c *ui.Context, key string, v *string, handle *ui.Handle, placeholder string) ui.Element {
	tok := tokens.Get(c)
	box := ui.Row(c).
		Height(tokens.ControlHeight).
		Radius(tokens.RadiusMd).
		AlignItems(ui.Center).
		PaddingX(10).
		Gap(8)

	focused := handle != nil && c.Focused(*handle)
	if focused {
		box.Background(tok.CardSurface).
			Border(tokens.BorderThin, tok.BrandPrimary).
			Shadow(0, 0, 0, 3, tok.FocusRing)
	} else {
		box.Background(tok.PageBg).
			Border(tokens.BorderThin, tok.BorderDefault)
	}

	box.Children(func() {
		input := ui.TextInputBase(c.Key(key), v).
			Grow(1).
			MinWidth(0).
			FontSize(13).
			Placeholder(placeholder).
			Label(placeholder)

		if handle != nil {
			input.Bind(handle)
		}
	})

	return box
}
