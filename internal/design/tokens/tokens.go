package tokens

import "github.com/egoist/mygo/ui"

// DefaultBrandPrimary is the app's default core brand color (Indigo 600).
var DefaultBrandPrimary = ui.Hex("#4f46e5")

// SemanticTokens defines the cross-platform modern neutral design tokens.
type SemanticTokens struct {
	Dark bool

	// Surface Layers (实体分层表面，零毛玻璃依赖，两端一致)
	PageBg       ui.Color // 页面底层背景
	CardSurface  ui.Color // 卡片主体表面
	CardElevated ui.Color // 悬浮弹窗/下拉菜单表面
	SidebarBg    ui.Color // 侧边栏背景

	// Typography (文字分级)
	TextPrimary   ui.Color // 主标题、正文文本
	TextSecondary ui.Color // 次要说明、表头标签
	TextMuted     ui.Color // 占位符、微弱辅助文字
	TextOnAccent  ui.Color // 强调色背景上的反白文字

	// Borders & Shadow (边框与阴影)
	BorderDefault ui.Color // 默认 1px 边框
	BorderMuted   ui.Color // 浅弱分割线
	Hairline      ui.Color // 极细半透明分割线
	ShadowColor   ui.Color // 实体卡片投影色

	// Brand & Interactive States (自动平滑派生)
	BrandPrimary ui.Color // 品牌核心色
	BrandHover   ui.Color // 悬停微过渡色
	BrandActive  ui.Color // 按压微过渡色
	Selection    ui.Color // 选中高亮底色 (低透明度)
	FocusRing    ui.Color // 键盘焦点外发光光环色

	// Semantic Statuses (功能状态色)
	Success ui.Color
	Warning ui.Color
	Danger  ui.Color
}

// DeriveTokens computes the semantic design tokens for the given appearance and brand color
// using perceptual color mixing for smooth hover and active transitions.
func DeriveTokens(isDark bool, brandPrimary ui.Color) *SemanticTokens {
	black, white := ui.Hex("#000000"), ui.Hex("#ffffff")

	var hover, active ui.Color
	if isDark {
		hover = brandPrimary.Mix(white, 0.15)
		active = brandPrimary.Mix(black, 0.15)
		return &SemanticTokens{
			Dark:          true,
			PageBg:        ui.Hex("#0d0f14"), // 深邃底色 (冷灰 Zinc/Slate)
			CardSurface:   ui.Hex("#16181f"), // 实体卡片表面
			CardElevated:  ui.Hex("#1e2230"), // 浮起层表面
			SidebarBg:     ui.Hex("#12141a"), // 侧边栏背景
			TextPrimary:   ui.Hex("#e8eaf0"),
			TextSecondary: ui.RGBA(232, 234, 240, 0.65),
			TextMuted:     ui.RGBA(232, 234, 240, 0.40),
			TextOnAccent:  ui.Hex("#ffffff"),
			BorderDefault: ui.Hex("#262a36"),
			BorderMuted:   ui.RGBA(255, 255, 255, 0.08),
			Hairline:      ui.RGBA(255, 255, 255, 0.05),
			ShadowColor:   ui.RGBA(0, 0, 0, 0.45),
			BrandPrimary:  brandPrimary,
			BrandHover:    hover,
			BrandActive:   active,
			Selection:     brandPrimary.Alpha(0.20),
			FocusRing:     brandPrimary.Alpha(0.45),
			Success:       ui.Hex("#4ade80"),
			Warning:       ui.Hex("#fbbf24"),
			Danger:        ui.Hex("#f87171"),
		}
	}

	hover = brandPrimary.Mix(black, 0.10)
	active = brandPrimary.Mix(black, 0.20)
	return &SemanticTokens{
		Dark:          false,
		PageBg:        ui.Hex("#f5f6f8"), // 干净冷灰底
		CardSurface:   ui.Hex("#ffffff"), // 纯白卡片表面
		CardElevated:  ui.Hex("#ffffff"),
		SidebarBg:     ui.Hex("#f0f2f5"),
		TextPrimary:   ui.Hex("#101828"),
		TextSecondary: ui.RGBA(16, 24, 40, 0.68),
		TextMuted:     ui.RGBA(16, 24, 40, 0.42),
		TextOnAccent:  ui.Hex("#ffffff"),
		BorderDefault: ui.Hex("#e4e7ec"),
		BorderMuted:   ui.RGBA(16, 24, 40, 0.07),
		Hairline:      ui.RGBA(0, 0, 0, 0.05),
		ShadowColor:   ui.RGBA(16, 24, 40, 0.06),
		BrandPrimary:  brandPrimary,
		BrandHover:    hover,
		BrandActive:   active,
		Selection:     brandPrimary.Alpha(0.10),
		FocusRing:     brandPrimary.Alpha(0.35),
		Success:       ui.Hex("#16a34a"),
		Warning:       ui.Hex("#d97706"),
		Danger:        ui.Hex("#dc2626"),
	}
}

var (
	cachedLight = DeriveTokens(false, DefaultBrandPrimary)
	cachedDark  = DeriveTokens(true, DefaultBrandPrimary)
)

// Get returns the semantic tokens matching the current context's appearance.
func Get(c *ui.Context) *SemanticTokens {
	if c.Theme() != nil && c.Theme().Dark {
		return cachedDark
	}
	return cachedLight
}

// GetByDark returns tokens for the specified appearance mode.
func GetByDark(dark bool) *SemanticTokens {
	if dark {
		return cachedDark
	}
	return cachedLight
}
