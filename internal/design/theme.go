package design

import (
	"sync"

	"mygo-app/internal/design/tokens"
	"mygo-app/internal/i18n"

	"github.com/egoist/mygo/ui"
)

// Mode is the app's appearance: the system's, or one forced by the user.
type Mode int

// The appearances the app offers.
const (
	ModeSystem Mode = iota
	ModeLight
	ModeDark
)

// OnModeChange, when set, runs after the user picks another appearance, so
// that the app can remember it. Set it in main.
var OnModeChange func(Mode)

var (
	mu        sync.Mutex
	mode      = ModeSystem
	systemDark bool
)

// Mode returns the appearance the user chose.
func CurrentMode() Mode {
	mu.Lock()
	defer mu.Unlock()
	return mode
}

// SetMode switches the appearance and tells the app, which saves it.
func SetMode(m Mode) {
	mu.Lock()
	changed := mode != m
	mode = m
	mu.Unlock()
	if changed && OnModeChange != nil {
		OnModeChange(m)
	}
}

// Label names a mode for the UI.
func (m Mode) Label() string {
	switch m {
	case ModeLight:
		return "浅色"
	case ModeDark:
		return "深色"
	default:
		return "跟随系统"
	}
}

// ParseMode reads what was saved, falling back to the system.
func ParseMode(s string) Mode {
	switch s {
	case "light":
		return ModeLight
	case "dark":
		return ModeDark
	default:
		return ModeSystem
	}
}

// MarshalText is the value the app saves.
func (m Mode) String() string {
	switch m {
	case ModeLight:
		return "light"
	case ModeDark:
		return "dark"
	default:
		return "system"
	}
}

// Look holds the colors the widgets of ui.Theme do not cover: the page
// behind the cards, the tint of the chosen navigation row, and the shadow
// of a card.
type Look struct {
	Page      ui.Color
	Card      ui.Color
	NavActive ui.Color
	Shadow    ui.Color
}

var (
	lightLook = Look{
		Page:      ui.Hex("#f5f6f8"),
		Card:      ui.Hex("#ffffff"),
		NavActive: ui.RGBA(79, 70, 229, 0.10),
		Shadow:    ui.RGBA(16, 24, 40, 0.06),
	}
	darkLook = Look{
		Page:      ui.Hex("#0d0f14"),
		Card:      ui.Hex("#16181f"),
		NavActive: ui.RGBA(129, 140, 248, 0.16),
		Shadow:    ui.RGBA(0, 0, 0, 0.45),
	}
	lightTheme *ui.Theme
	darkTheme  *ui.Theme
)

// Look returns the colors of the current appearance, given the frame's
// theme, which is either ours already or the platform's on the first frame.
func LookOf(dark bool) Look {
	if dark {
		return darkLook
	}
	return lightLook
}

// Dark reports whether the current appearance is dark, given whether the
// system is.
func Dark(systemIsDark bool) bool {
	switch CurrentMode() {
	case ModeLight:
		return false
	case ModeDark:
		return true
	default:
		return systemIsDark
	}
}

// Theme returns the app's theme. It is built once per appearance from the
// palettes below, not from the platform's, so that both appearances look
// like this app and not like the OS.
func Theme(c *ui.Context) *ui.Theme {
	systemDark = c.Theme().Dark
	if dark := Dark(systemDark); dark {
		if darkTheme == nil {
			darkTheme = darkPalette()
		}
		return darkTheme
	}
	if lightTheme == nil {
		lightTheme = lightPalette()
	}
	return lightTheme
}

// Look returns the extra colors that go with Theme.
func LookFor(c *ui.Context) Look { return LookOf(Theme(c).Dark) }

// Tokens returns the cross-platform modern neutral design tokens matching the current appearance.
func Tokens(c *ui.Context) *tokens.SemanticTokens {
	return tokens.GetByDark(Theme(c).Dark)
}

func lightPalette() *ui.Theme {
	return &ui.Theme{
		Background:     ui.Hex("#ffffff"),
		Surface:        ui.Hex("#f5f6f8"),
		SurfaceHover:   ui.Hex("#eceef3"),
		SurfacePressed: ui.Hex("#e1e4ea"),
		Border:         ui.Hex("#e4e7ec"),
		Text:           ui.Hex("#101828"),
		TextMuted:      ui.Hex("#667085"),
		Accent:         ui.Hex("#4f46e5"),
		AccentHover:    ui.Hex("#4338ca"),
		AccentPressed:  ui.Hex("#3730a3"),
		AccentText:     ui.Hex("#ffffff"),
		Danger:         ui.Hex("#dc2626"),
		Warning:        ui.Hex("#d97706"),
		Success:        ui.Hex("#16a34a"),
		Selection:      ui.RGBA(79, 70, 229, 0.18),
		Focus:          ui.RGBA(79, 70, 229, 0.45),
		Scrollbar:      ui.RGBA(16, 24, 40, 0.22),
		Radius:         10,
		Spacing:        4,
		FontSize:       13,
	}
}

func darkPalette() *ui.Theme {
	return &ui.Theme{
		Dark:           true,
		Background:     ui.Hex("#16181f"),
		Surface:        ui.Hex("#0d0f14"),
		SurfaceHover:   ui.Hex("#1f222c"),
		SurfacePressed: ui.Hex("#282c38"),
		Border:         ui.Hex("#262a36"),
		Text:           ui.Hex("#e8eaf0"),
		TextMuted:      ui.Hex("#949bab"),
		Accent:         ui.Hex("#6366f1"),
		AccentHover:    ui.Hex("#818cf8"),
		AccentPressed:  ui.Hex("#4f46e5"),
		AccentText:     ui.Hex("#ffffff"),
		Danger:         ui.Hex("#f87171"),
		Warning:        ui.Hex("#fbbf24"),
		Success:        ui.Hex("#4ade80"),
		Selection:      ui.RGBA(99, 102, 241, 0.30),
		Focus:          ui.RGBA(129, 140, 248, 0.55),
		Scrollbar:      ui.RGBA(232, 234, 240, 0.18),
		Radius:         10,
		Spacing:        4,
		FontSize:       13,
	}
}

// Appearance is the three-way control of the app's look, in 系统设置.
func Appearance(c *ui.Context) {
	chosen := int(CurrentMode())
	if ui.Segmented(c, &chosen, i18n.T("settings.theme_auto"), i18n.T("settings.theme_light"), i18n.T("settings.theme_dark")).Changed() {
		SetMode(Mode(chosen))
	}
}

// LanguageSelector provides the segmented toggle for application language in 系统设置.
func LanguageSelector(c *ui.Context) {
	chosen := 0
	if i18n.Current() == i18n.LangEN {
		chosen = 1
	}
	if ui.Segmented(c, &chosen, "简体中文", "English").Changed() {
		if chosen == 0 {
			i18n.SetLanguage(i18n.LangZH)
		} else {
			i18n.SetLanguage(i18n.LangEN)
		}
	}
}
