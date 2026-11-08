package components

import (
	"mygo-app/internal/design/tokens"

	"github.com/egoist/mygo/ui"
)

// Heading displays a page or section title with tuned kerning (negative letter spacing
// for the large title, slight positive tracking for the subtitle).
func Heading(c *ui.Context, title, sub string) {
	tok := tokens.Get(c)
	ui.Column(c).Gap(tokens.SpaceXs).Margin(0, 0, tokens.SpaceLg, 0).Children(func() {
		if sub != "" {
			ui.Text(c, sub).
				FontSize(12).
				FontWeight(600).
				TextColor(tok.TextMuted).
				LetterSpacing(0.3).
				SingleLine()
		}
		ui.Text(c, title).
			FontSize(22).
			FontWeight(700).
			LetterSpacing(-0.4).
			TextColor(tok.TextPrimary).
			SingleLine()
	})
}

// NumText renders text with OpenType tabular numbers (tnum) enabled, preventing
// numbers or timer counters from wobbling when values change.
func NumText(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontFeatures("tnum")
}
