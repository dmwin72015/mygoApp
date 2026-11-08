package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mygo-app/internal/config"
	"mygo-app/internal/design"
	"mygo-app/internal/design/tokens"
	"mygo-app/internal/i18n"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var (
	settingsWinMu sync.Mutex
	settingsWin   *mygo.Window
	settingsState *settingsDialog
)

type settingsTab int

const (
	tabDataSource settingsTab = iota
	tabAppearance
	tabLanguage
	tabStorage
	tabNetwork
	tabAbout
)

type settingsDialog struct {
	tab settingsTab

	// Data Source fields
	driverChoice int // 0: SQLite, 1: Remote REST API
	apiURL       string
	sqlitePath   string
	pingResult   string
	pingSuccess  bool
	pingBusy     bool
	saveMsg      string
	saveErr      string

	// Network / Proxy fields
	proxyMode int // 0: Direct, 1: System, 2: Custom HTTP
	proxyURL  string

	onSaved func()
}

// OpenSettingsWindow opens or focuses the standalone settings window.
func OpenSettingsWindow(parent *mygo.Window, onSaved func()) {
	settingsWinMu.Lock()
	defer settingsWinMu.Unlock()

	if settingsWin != nil && !settingsWin.IsDestroyed() {
		settingsWin.Show()
		settingsWin.Focus()
		return
	}

	state := &settingsDialog{
		onSaved: onSaved,
	}
	state.loadCurrentConfig()
	settingsState = state

	settingsWin = mygo.NewWindow(mygo.WindowOptions{
		Title:           i18n.T("settings.title") + " · Preferences",
		Width:           680,
		Height:          520,
		MinWidth:        600,
		MinHeight:       460,
		DisableMaximize: true,
		Parent:          parent,
		StateKey:        "settings_window",
		Content:         ui.View(state.view),
	})

	settingsWin.OnClose(func(e *mygo.CloseEvent) {
		settingsWinMu.Lock()
		settingsWin = nil
		settingsState = nil
		settingsWinMu.Unlock()
	})
}

func (s *settingsDialog) loadCurrentConfig() {
	cfg, err := config.LoadClient("configs/client.yaml")
	if err != nil {
		cfg = config.DefaultClient()
	}

	if cfg.Driver == "remote" {
		s.driverChoice = 1
	} else {
		s.driverChoice = 0
	}
	s.apiURL = cfg.APIURL
	if s.apiURL == "" {
		s.apiURL = "http://127.0.0.1:8099"
	}
	s.sqlitePath = cfg.SQLitePath
	if s.sqlitePath == "" {
		s.sqlitePath = "school.db"
	}
}

func (s *settingsDialog) testConnection() {
	if s.pingBusy {
		return
	}
	target := strings.TrimRight(strings.TrimSpace(s.apiURL), "/")
	if target == "" {
		s.pingResult = "请输入有效的基础服务地址 (URL)"
		s.pingSuccess = false
		return
	}

	s.pingBusy = true
	s.pingResult = "正在测试连接…"
	s.pingSuccess = false

	go func() {
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target+"/ping", nil)
		if err != nil {
			s.finishPing(fmt.Sprintf("请求构建失败: %v", err), false)
			return
		}

		res, err := http.DefaultClient.Do(req)
		elapsed := time.Since(start).Milliseconds()

		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "connection refused") {
				msg = "连接被拒绝 (Connection Refused)，请确认后端服务已启动"
			}
			s.finishPing(fmt.Sprintf("无法连接 (%dms): %s", elapsed, msg), false)
			return
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)

		if res.StatusCode == http.StatusOK {
			s.finishPing(fmt.Sprintf("✓ 连接成功 (HTTP 200，延时 %dms)", elapsed), true)
		} else {
			s.finishPing(fmt.Sprintf("服务端返回状态码 %d: %s", res.StatusCode, string(body)), false)
		}
	}()
}

func (s *settingsDialog) finishPing(msg string, success bool) {
	if settingsWin != nil {
		settingsWin.Update(func() {
			s.pingBusy = false
			s.pingResult = msg
			s.pingSuccess = success
		})
	}
}

func (s *settingsDialog) saveConfig() {
	cfg := &config.ClientConfig{
		APIURL:     strings.TrimRight(strings.TrimSpace(s.apiURL), "/"),
		SQLitePath: strings.TrimSpace(s.sqlitePath),
	}
	if s.driverChoice == 1 {
		cfg.Driver = "remote"
	} else {
		cfg.Driver = "sqlite"
	}

	if err := config.SaveClient("configs/client.yaml", cfg); err != nil {
		s.saveErr = fmt.Sprintf("保存失败: %v", err)
		s.saveMsg = ""
		return
	}

	s.saveErr = ""
	s.saveMsg = "✓ 配置已保存至 configs/client.yaml（重启客户端生效）"
	if s.onSaved != nil {
		s.onSaved()
	}
}

func (s *settingsDialog) view(c *ui.Context) {
	c.SetTheme(design.Theme(c))
	t := design.Theme(c)
	look := design.LookFor(c)

	ui.Row(c).Fill().Background(look.Page).AlignItems(ui.Stretch).Children(func() {
		// 1. Left Navigation Tabs
		ui.Column(c).Width(150).Shrink(0).Padding(14, 10).Gap(4).
			Background(look.Card).BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
			ui.Text(c, "设置 · Preferences").FontSize(12).TextColor(t.TextMuted).Bold().Padding(4, 6, 8)

			s.tabButton(c, tabDataSource, "🌐 数据源", "Data Source")
			s.tabButton(c, tabAppearance, "🎨 外观主题", "Appearance")
			s.tabButton(c, tabLanguage, "🌍 语言地区", "Language")
			s.tabButton(c, tabStorage, "📁 本地存储", "Storage")
			s.tabButton(c, tabNetwork, "🔌 网络代理", "Network")
			s.tabButton(c, tabAbout, "ℹ️ 关于系统", "About")
		})

		// 2. Right Content Panel
		ui.Column(c).Grow(1).Padding(24, 28).Gap(16).Children(func() {
			switch s.tab {
			case tabDataSource:
				s.renderDataSourceTab(c, t)
			case tabAppearance:
				s.renderAppearanceTab(c, t)
			case tabLanguage:
				s.renderLanguageTab(c, t)
			case tabStorage:
				s.renderStorageTab(c, t)
			case tabNetwork:
				s.renderNetworkTab(c, t)
			case tabAbout:
				s.renderAboutTab(c, t)
			}
		})
	})
}

func (s *settingsDialog) tabButton(c *ui.Context, tab settingsTab, titleZH, titleEN string) {
	active := s.tab == tab
	label := titleZH
	if i18n.Current() == i18n.LangEN {
		label = titleEN
	}
	if design.NavRow(c, label, active) {
		s.tab = tab
		s.saveMsg, s.saveErr = "", ""
	}
}

func (s *settingsDialog) renderDataSourceTab(c *ui.Context, t *ui.Theme) {
	tok := tokens.Get(c)
	ui.Column(c).Gap(14).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "数据源与服务连接 (Data Source)").FontSize(18).Bold()
			ui.Text(c, "选择系统数据保存模式：本地离线单机运行或通过网络连接业务后端。").
				FontSize(12).TextColor(t.TextMuted)
		})

		design.Card(c, func() {
			ui.Column(c).Gap(12).Children(func() {
				ui.Text(c, "数据模式选择").FontSize(13).Bold()
				ui.Segmented(c, &s.driverChoice, "本地 SQLite 模式", "远程 Server API 模式")

				if s.driverChoice == 1 {
					ui.Field(c, "服务端 API 地址 (Base URL)", func() {
						ui.TextInput(c, &s.apiURL).Placeholder("http://127.0.0.1:8099")
					})
					ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
						if ui.Button(c, "测试连接 (Ping)").Disabled(s.pingBusy).Clicked() {
							s.testConnection()
						}
						if s.pingResult != "" {
							color := tok.Danger
							if s.pingSuccess {
								color = tok.Success
							}
							ui.Text(c, s.pingResult).FontSize(12).TextColor(color).Bold()
						}
					})
				} else {
					ui.Field(c, "SQLite 数据库文件路径", func() {
						ui.TextInput(c, &s.sqlitePath).Placeholder("school.db")
					})
				}
			})
		})

		if s.saveMsg != "" {
			ui.Text(c, s.saveMsg).FontSize(12).TextColor(tok.Success).Bold()
		}
		if s.saveErr != "" {
			ui.Text(c, s.saveErr).FontSize(12).TextColor(tok.Danger).Bold()
		}

		ui.Row(c).Justify(ui.End).Children(func() {
			if ui.PrimaryButton(c, "保存配置 (Save)").Clicked() {
				s.saveConfig()
			}
		})
	})
}

func (s *settingsDialog) renderAppearanceTab(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(14).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "外观与主题 (Appearance)").FontSize(18).Bold()
			ui.Text(c, "自定义界面明暗显示，随心切换夜间模式或跟随操作系统。").
				FontSize(12).TextColor(t.TextMuted)
		})

		design.Card(c, func() {
			ui.Column(c).Gap(12).Children(func() {
				ui.Text(c, "界面显示模式").FontSize(13).Bold()
				design.Appearance(c)
				ui.Text(c, "支持跟随操作系统无缝平滑切换深色/浅色样式。").
					FontSize(11).TextColor(t.TextMuted)
			})
		})
	})
}

func (s *settingsDialog) renderLanguageTab(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(14).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "多语言与区域 (Language)").FontSize(18).Bold()
			ui.Text(c, "即时切换系统语言，双端界面与接口文案无缝响应。").
				FontSize(12).TextColor(t.TextMuted)
		})

		design.Card(c, func() {
			ui.Column(c).Gap(12).Children(func() {
				ui.Text(c, "当前语言偏好").FontSize(13).Bold()
				design.LanguageSelector(c)
				ui.Text(c, "切换后即刻对所有窗口与弹层生效。").
					FontSize(11).TextColor(t.TextMuted)
			})
		})
	})
}

func (s *settingsDialog) renderStorageTab(c *ui.Context, t *ui.Theme) {
	dataDir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil || dataDir == "" {
		dataDir = "."
	}
	dbFile := filepath.Join(dataDir, "school.db")

	ui.Column(c).Gap(14).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "存储与本地文件 (Storage)").FontSize(18).Bold()
			ui.Text(c, "查看本地数据库与应用配置文件的物理存储位置。").
				FontSize(12).TextColor(t.TextMuted)
		})

		design.Card(c, func() {
			ui.Column(c).Gap(12).Children(func() {
				ui.Text(c, "用户数据目录").FontSize(13).Bold()
				ui.Text(c, dataDir).FontSize(11).TextColor(t.TextMuted)

				ui.Text(c, "SQLite 数据库路径").FontSize(13).Bold()
				ui.Text(c, dbFile).FontSize(11).TextColor(t.TextMuted)

				ui.Row(c).Gap(10).Children(func() {
					if ui.Button(c, "在访达/资源管理器中打开").Clicked() {
						_ = mygo.Shell.OpenPath(dataDir)
					}
					if ui.Button(c, "打开配置目录 (configs)").Clicked() {
						_ = mygo.Shell.OpenPath("configs")
					}
				})
			})
		})
	})
}

func (s *settingsDialog) renderNetworkTab(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(14).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "网络与代理 (Network & Proxy)").FontSize(18).Bold()
			ui.Text(c, "配置客户端访问远程服务端时的网络路由与代理服务器。").
				FontSize(12).TextColor(t.TextMuted)
		})

		design.Card(c, func() {
			ui.Column(c).Gap(12).Children(func() {
				ui.Text(c, "网络代理模式").FontSize(13).Bold()
				ui.Segmented(c, &s.proxyMode, "直连模式 (Direct)", "使用系统代理", "自定义代理")

				if s.proxyMode == 2 {
					ui.Field(c, "HTTP 代理地址", func() {
						ui.TextInput(c, &s.proxyURL).Placeholder("http://127.0.0.1:7890")
					})
				} else {
					ui.Text(c, "默认采用系统网络协议栈直接通信。").
						FontSize(11).TextColor(t.TextMuted)
				}
			})
		})
	})
}

func (s *settingsDialog) renderAboutTab(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Gap(14).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, "关于系统 (About)").FontSize(18).Bold()
			ui.Text(c, "启明培训 · 现代化企业级教培管理后台。").
				FontSize(12).TextColor(t.TextMuted)
		})

		design.Card(c, func() {
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, "启明培训 · 管理后台").FontSize(15).Bold()
				ui.Text(c, "版本: v1.2.0 (Build 2026.11)").FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, "核心框架: Go 1.24 + MyGo Native GUI Framework").FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, "数据库支持: PostgreSQL 16+ & SQLite 3").FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, "架构特性: C/S 架构，双端国际化，支持离线单机与云端协同").FontSize(12).TextColor(t.TextMuted)
			})
		})
	})
}
