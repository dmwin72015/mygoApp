// Package tokens defines the design system's primitive metrics, scales, and
// semantic tokens for cross-platform modern desktop applications.
package tokens

const (
	// Radius scale (跨平台黄金中性圆角)
	RadiusSm   float32 = 6   // Badge, Checkbox, 内部指示滑块
	RadiusMd   float32 = 8   // 按钮, 输入框, 选择器, 分段控制器外框
	RadiusLg   float32 = 10  // 卡片, 表格容器, 模态弹窗面板
	RadiusPill float32 = 999 // 圆形图标按钮或小胶囊状态条

	// Spacing and gap scale
	SpaceXs  float32 = 4
	SpaceSm  float32 = 8
	SpaceMd  float32 = 12
	SpaceLg  float32 = 16
	SpaceXl  float32 = 24
	Space2Xl float32 = 32

	// Standard layout dimensions (Heights & Widths)
	ControlHeight float32 = 34  // 标准交互控件高度 (按钮, 输入框)
	HeaderHeight  float32 = 48  // 顶部全局工具栏高度
	SidebarWidth  float32 = 216 // 左侧导航侧边栏宽度
	BorderThin    float32 = 1   // 统一 1px 实体发丝边框
)
