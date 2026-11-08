package components_test

import (
	"testing"

	"mygo-app/internal/design/components"

	"github.com/egoist/mygo/ui"
)

func TestActionButtonClick(t *testing.T) {
	clicked := false
	tt := ui.NewTester(func(c *ui.Context) {
		if components.ActionButton(c, "保存学员", components.ButtonPrimary).Clicked() {
			clicked = true
		}
	}, 400, 300)

	if err := tt.Click("保存学员"); err != nil {
		t.Fatalf("failed to click ActionButton: %v", err)
	}
	tt.Frame()

	if !clicked {
		t.Fatal("expected ActionButton.Clicked() to be true after click")
	}
}

func TestSmoothSwitchToggle(t *testing.T) {
	on := false
	tt := ui.NewTester(func(c *ui.Context) {
		components.SmoothSwitch(c, &on).Label("自动排课")
	}, 400, 300)

	if err := tt.Click("自动排课"); err != nil {
		t.Fatalf("failed to click switch: %v", err)
	}
	tt.Frame()

	if !on {
		t.Fatal("expected switch to toggle on=true")
	}
}

func TestSegmentedPicker(t *testing.T) {
	selected := 0
	options := []string{"全部", "在读", "结业"}
	changed := false

	tt := ui.NewTester(func(c *ui.Context) {
		if components.SegmentedPicker(c, "status_filter", &selected, options) {
			changed = true
		}
	}, 500, 300)

	if err := tt.Click("在读"); err != nil {
		t.Fatalf("failed to click segment '在读': %v", err)
	}
	tt.Frame()

	if selected != 1 {
		t.Fatalf("expected selected=1, got %d", selected)
	}
	if !changed {
		t.Fatal("expected SegmentedPicker to return changed=true")
	}
}

func TestContainersAndTypography(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		components.Heading(c, "学员管理", "2026年秋季学期")
		components.ModernCard(c, func() {
			components.NumText(c, "¥1,280.00")
			components.StatusBadge(c, "正常在读", components.ToneSuccess)
		})
	}, 500, 400)

	if _, ok := tt.Find("学员管理"); !ok {
		t.Fatal("heading title not found in layout")
	}
	if _, ok := tt.Find("正常在读"); !ok {
		t.Fatal("status badge not found in layout")
	}
}

func TestModernTableSelection(t *testing.T) {
	type item struct {
		name   string
		status string
	}
	items := []item{
		{name: "张小凡", status: "在读"},
		{name: "林惊羽", status: "结业"},
	}

	selected := 0
	var tbl ui.ListState
	tbl.Selected = &selected

	cols := []ui.TableColumn{
		{Title: "姓名", Width: 120},
		{Title: "状态", Width: 80},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		components.ModernTable(c, &tbl, cols, len(items), func(row, col int) {
			it := items[row]
			switch col {
			case 0:
				ui.Text(c, it.name).SingleLine()
			case 1:
				components.StatusBadge(c, it.status, components.ToneSuccess)
			}
		}).Grow(1)
	}, 600, 400)

	// Ensure texts are present
	if _, ok := tt.Find("张小凡"); !ok {
		t.Fatal("row 0 text not found")
	}
	if _, ok := tt.Find("林惊羽"); !ok {
		t.Fatal("row 1 text not found")
	}

	// Click row 1
	if err := tt.Click("林惊羽"); err != nil {
		t.Fatalf("failed to click row 1: %v", err)
	}
	tt.Frame()

	if selected != 1 {
		t.Fatalf("expected selected=1 after click, got %d", selected)
	}

	// Verify the background color of the selected row is light/soft (not dark solid indigo)
	rect, ok := tt.Find("林惊羽")
	if !ok {
		t.Fatal("row 1 not found after click")
	}
	img := tt.Image()
	pxColor := img.At(int(rect.X-10), int(rect.Y+rect.H/2))
	r, g, b, _ := pxColor.RGBA()
	// RGBA() returns alpha-premultiplied uint32 in [0, 0xffff]
	r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)
	// Dark indigo #4f46e5 has r=79, g=70. A soft light selection tint has r>200, g>200.
	if r8 < 200 || g8 < 200 {
		t.Fatalf("expected light background tint for selected row, got rgb(%d, %d, %d)", r8, g8, b8)
	}
	// Test in Dark Mode as well
	selectedDark := 0
	var tblDark ui.ListState
	tblDark.Selected = &selectedDark

	ttDark := ui.NewTester(func(c *ui.Context) {
		components.ModernTable(c, &tblDark, cols, len(items), func(row, col int) {
			it := items[row]
			switch col {
			case 0:
				ui.Text(c, it.name).SingleLine()
			case 1:
				components.StatusBadge(c, it.status, components.ToneSuccess)
			}
		}).Grow(1)
	}, 600, 400)
	ttDark.SetDark(true)
	ttDark.Frame()

	rectDark, ok := ttDark.Find("张小凡")
	if !ok {
		t.Fatal("dark mode row 0 not found")
	}
	pxDark := ttDark.Image().At(int(rectDark.X-10), int(rectDark.Y+rectDark.H/2))
	rd, gd, bd, _ := pxDark.RGBA()
	rd8, gd8, bd8 := uint8(rd>>8), uint8(gd>>8), uint8(bd>>8)
	// In dark mode, background should be subtle dark slate-indigo (~20-50), not bright or pure black
	if rd8 > 100 || gd8 > 100 || bd8 < 20 {
		t.Fatalf("unexpected dark mode selection tint: rgb(%d, %d, %d)", rd8, gd8, bd8)
	}

	// Test double-click submit
	opened := -1
	ttSubmit := ui.NewTester(func(c *ui.Context) {
		if components.ModernTable(c, &tbl, cols, len(items), func(row, col int) {
			it := items[row]
			switch col {
			case 0:
				ui.Text(c, it.name).SingleLine()
			case 1:
				components.StatusBadge(c, it.status, components.ToneSuccess)
			}
		}).Grow(1).Submitted() {
			opened = selected
		}
	}, 600, 400)

	_ = ttSubmit.Click("张小凡")
	_ = ttSubmit.Click("张小凡")
	ttSubmit.Frame()
	if opened != 0 {
		t.Fatalf("expected double click to submit row 0, got opened=%d", opened)
	}
}

