package design_test

import (
	"image/png"
	"os"
	"testing"

	"mygo-app/internal/design"

	"github.com/egoist/mygo/ui"
)

// TestSnapshot writes a PNG of the design system in both appearances, to
// look at what the theme and the widgets actually produce:
//
//	MYGO_SNAPSHOT=1 go test ./internal/design -run Snapshot
//
// It is skipped otherwise, so a normal test run writes no files.
func TestSnapshot(t *testing.T) {
	if os.Getenv("MYGO_SNAPSHOT") == "" {
		t.Skip("set MYGO_SNAPSHOT=1 to write the PNGs")
	}
	for _, m := range []design.Mode{design.ModeLight, design.ModeDark} {
		design.SetMode(m)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(design.Theme(c))
			board(c)
		}, 980, 620)
		tt.Frame()
		tt.Frame()
		f, err := os.Create("/tmp/board-" + m.String() + ".png")
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote /tmp/board-%s.png", m)
	}
	design.SetMode(design.ModeSystem)
}

// board is a page that uses every piece of the design system at once, so
// that one picture shows the whole look.
func board(c *ui.Context) {
	t, look := design.Theme(c), design.LookFor(c)
	ui.Row(c).Fill().Background(look.Page).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(design.SideWidth).Shrink(0).Padding(14, 10).Gap(2).
			Background(design.LookFor(c).Card).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
			ui.Row(c).Padding(0, 6, 18).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "启明培训").FontSize(15).Bold()
				ui.Spacer(c)
				design.Pill(c, "后台", design.ToneAccent)
			})
			ui.Text(c, "业务").FontSize(11).TextColor(t.TextMuted).Padding(10, 10, 4)
			for _, it := range []struct {
				label string
				on    bool
			}{
				{"概览", true}, {"学生管理", false}, {"老师管理", false},
				{"教室管理", false}, {"课程管理", false}, {"排课", false}, {"充值 / 消课", false},
			} {
				design.NavRow(c, it.label, it.on)
			}
			ui.Spacer(c)
			ui.Row(c).Padding(6, 4, 0).Gap(8).AlignItems(ui.Center).
				BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
				ui.Box(c).Size(26, 26).Radius(13).Background(t.SurfacePressed).Center().Children(func() {
					ui.Text(c, "管").FontSize(11).Bold()
				})
				ui.Column(c).Grow(1).Gap(1).MinWidth(0).Children(func() {
					ui.Text(c, "管理员").FontSize(12).SingleLine()
					ui.Text(c, "管理员 · 已登录").FontSize(10).TextColor(t.TextMuted)
				})
				ui.Button(c, "退出")
			})
		})
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			design.Page(c, design.PageProps{
				Title:    "概览",
				Subtitle: "今天该做什么，一眼看到",
				Actions: func() {
					ui.Button(c, "刷新")
					ui.PrimaryButton(c, "新增学生")
				},
			}, func() {
				ui.Row(c).Gap(design.Gap).AlignItems(ui.Stretch).Children(func() {
					for _, s := range [][3]string{
						{"在读学生", "128", "名学员"},
						{"在职老师", "9", "位老师"},
						{"在售课程", "14", "门课程"},
						{"预存余额", "¥86,400.00", "尚未消课"},
					} {
						s := s
						ui.Box(c).Grow(1).MinWidth(0).Children(func() {
							design.Stat(c, s[0], s[1], s[2])
						})
					}
				})
				design.SectionCard(c, "最近流水", nil, func() {
					for _, row := range []struct {
						name string
						tone design.Tone
						note string
						amt  string
					}{
						{"王小明", design.TonePositive, "报名初二数学同步", "+¥4,800.00"},
						{"李思雨", design.ToneWarning, "消课：少儿英语 L12", "-¥180.00"},
						{"张浩然", design.TonePositive, "预存", "+¥2,000.00"},
						{"王小明", design.ToneWarning, "消课：初二数学 L07", "-¥200.00"},
					} {
						row := row
						ui.Row(c).Gap(10).AlignItems(ui.Center).PaddingY(5).Children(func() {
							label := "充值"
							if row.tone == design.ToneWarning {
								label = "消课"
							}
							design.Pill(c, label, row.tone)
							ui.Text(c, row.name).FontSize(13).Width(90).SingleLine()
							ui.Text(c, row.note).FontSize(12).TextColor(t.TextMuted).Grow(1).SingleLine()
							ui.Text(c, row.amt).FontSize(12).Bold()
						})
					}
				})
				design.SectionCard(c, "学员名录（选中态演示）", nil, func() {
					type stu struct {
						name, phone, balance, status string
						tone                          design.Tone
					}
					stus := []stu{
						{"张小凡", "13800138000", "¥3,200.00", "在读", design.TonePositive},
						{"陆雪琪", "13900139000", "¥5,600.00", "待排课", design.ToneAccent},
						{"林惊羽", "13700137000", "¥180.00", "余额不足", design.ToneWarning},
					}
					sel := 0
					var tbl ui.ListState
					tbl.Selected = &sel
					cols := []ui.TableColumn{
						{Title: "姓名", Width: 110},
						{Title: "联系电话", Width: 130},
						{Title: "预存余额", Width: 110, Align: ui.End},
						{Title: "状态", Width: 90},
					}
					design.Table(c, &tbl, cols, len(stus), func(row, col int) {
						v := stus[row]
						switch col {
						case 0:
							ui.Text(c, v.name).SingleLine()
						case 1:
							ui.Text(c, v.phone).SingleLine()
						case 2:
							ui.Text(c, v.balance).SingleLine()
						case 3:
							design.Pill(c, v.status, v.tone)
						}
					}).Height(130)
				})
				design.SectionCard(c, "状态", nil, func() {
					ui.Row(c).Gap(8).Children(func() {
						design.Pill(c, "在读", design.TonePositive)
						design.Pill(c, "待上课", design.ToneAccent)
						design.Pill(c, "已取消", design.ToneDanger)
						design.Pill(c, "停用", design.ToneNeutral)
						design.Pill(c, "余额不足", design.ToneWarning)
					})
					ui.SearchField(c, new(string)).Placeholder("搜索姓名、家长或电话").Width(240)
				})
			})
		})
	})
}
