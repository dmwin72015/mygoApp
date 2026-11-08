// Package client implements the desktop GUI application.
package client

import (
	"context"
	"log"
	"path/filepath"
	"runtime"
	"strings"

	"mygo-app/internal/config"
	"mygo-app/internal/data"
	"mygo-app/internal/design"
	"mygo-app/internal/domain"
	"mygo-app/internal/features"
	"mygo-app/internal/i18n"
	"mygo-app/internal/shell"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// root owns what outlives a page: the store, the session, the shell and
// the quit confirmation.
type root struct {
	win     *mygo.Window
	repo    domain.Repo
	login   *features.Login
	user    *domain.User
	sh      *shell.Shell
	openErr error

	askingQuit bool
	quitting   bool
}

func (r *root) view(c *ui.Context) {
	c.SetTheme(design.Theme(c))
	r.screen(c)
	if ui.AlertDialog(c, &r.askingQuit, i18n.T("app.quit.title"),
		i18n.T("app.quit.desc"), i18n.T("btn.cancel"), i18n.T("btn.quit")) == 1 {
		r.quitting = true
		mygo.App.Quit()
	}
}

func (r *root) screen(c *ui.Context) {
	switch {
	case r.openErr != nil:
		design.Page(c, design.PageProps{Title: i18n.T("app.err_open_store")}, func() {
			design.ErrorBar(c, r.openErr)
			ui.Text(c, i18n.T("app.err_hint")).
				FontSize(12).TextColor(design.Theme(c).TextMuted)
			if ui.Button(c, "⚙ "+i18n.T("settings.title")).Height(30).Clicked() {
				OpenSettingsWindow(r.win, nil)
			}
		})
	case r.repo == nil:
		design.Page(c, design.PageProps{Title: i18n.T("app.loading")}, func() {
			design.Loading(c, i18n.T("app.load_data"))
		})
	case r.user == nil:
		r.login.View(c)
	default:
		r.sh.View(c)
	}
}

func (r *root) signedIn(u domain.User) {
	r.user = &u
	if r.sh == nil {
		r.sh = shell.New(u)
		r.sh.OnSignOut = r.signOut
		r.sh.OnOpenSettings = func() {
			OpenSettingsWindow(r.win, nil)
		}
		r.sh.Attach(r.repo, r.win)
	}
	r.win.Invalidate()
}

func (r *root) signOut() {
	r.user, r.sh = nil, nil
	r.win.Invalidate()
}

func (r *root) restoreAppearance(repo domain.Repo) {
	ctx := context.Background()
	if saved, err := repo.Settings().Get(ctx, "appearance"); err == nil {
		design.SetMode(design.ParseMode(saved))
	}
	design.OnModeChange = func(m design.Mode) {
		go func() { _ = repo.Settings().Set(context.Background(), "appearance", m.String()) }()
	}
}

func (r *root) restoreLanguage(repo domain.Repo) {
	ctx := context.Background()
	if saved, err := repo.Settings().Get(ctx, "language"); err == nil && saved != "" {
		i18n.SetLanguage(saved)
	}
	i18n.OnLanguageChange = func(lang string) {
		go func() { _ = repo.Settings().Set(context.Background(), "language", lang) }()
		if r.win != nil {
			setupAppMenu(r.win)
			r.win.Invalidate()
		}
	}
}

// Options specifies runtime configuration options for the desktop client.
type Options struct {
	ConfigFile string
	Driver     string
	APIURL     string
	SQLitePath string
	Token      string
}

// Run launches the desktop GUI client application with default settings.
func Run() {
	RunWithOptions(Options{})
}

// RunWithOptions launches the desktop GUI client application with specific options.
func RunWithOptions(opts Options) {
	r := &root{}
	mygo.App.WhenReady(func() {
		r.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "启明培训 · 管理后台",
			Width:     1180,
			Height:    760,
			MinWidth:  980,
			MinHeight: 620,
			StateKey:  "main",
			Content:   ui.View(r.view),
		})

		setupAppMenu(r.win)

		cfg := resolveConfig(opts)
		log.Printf("[CLIENT] 正在连接数据源: driver=%s, endpoint=%s", cfg.Driver, cfg.DSN)

		repo, err := data.Open(context.Background(), cfg)
		if err != nil {
			r.openErr = err
			log.Printf("open store: %v", err)
		} else {
			r.repo = repo
			r.login = features.NewLogin(repo, r.win, r.signedIn)
			r.login.OnOpenSettings = func() {
				OpenSettingsWindow(r.win, nil)
			}
			r.restoreAppearance(repo)
			r.restoreLanguage(repo)
		}

		r.win.OnClose(func(e *mygo.CloseEvent) {
			if r.quitting {
				return
			}
			e.PreventDefault()
			r.askingQuit = true
			r.win.Invalidate()
		})
		r.win.Invalidate()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func buildAppMenu(win *mygo.Window) *mygo.Menu {
	appSubmenu := []*mygo.MenuItem{
		{Role: mygo.RoleAbout},
		mygo.Separator(),
		{
			Label:       i18n.T("settings.title"),
			Accelerator: "CmdOrCtrl+,",
			Click: func(item *mygo.MenuItem, w *mygo.Window) {
				OpenSettingsWindow(win, nil)
			},
		},
		mygo.Separator(),
		{Role: mygo.RoleServices},
		mygo.Separator(),
		{Role: mygo.RoleHide},
		{Role: mygo.RoleHideOthers},
		{Role: mygo.RoleUnhide},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	}

	fileSubmenu := []*mygo.MenuItem{}
	if runtime.GOOS != "darwin" {
		fileSubmenu = append(fileSubmenu,
			&mygo.MenuItem{
				Label:       i18n.T("settings.title"),
				Accelerator: "CmdOrCtrl+,",
				Click: func(item *mygo.MenuItem, w *mygo.Window) {
					OpenSettingsWindow(win, nil)
				},
			},
			mygo.Separator(),
		)
	}
	fileSubmenu = append(fileSubmenu, &mygo.MenuItem{Role: mygo.RoleClose})
	if runtime.GOOS != "darwin" {
		fileSubmenu = append(fileSubmenu,
			mygo.Separator(),
			&mygo.MenuItem{Role: mygo.RoleQuit},
		)
	}

	return mygo.NewMenu([]*mygo.MenuItem{
		{
			Role:    mygo.RoleAppMenu,
			Submenu: appSubmenu,
		},
		{
			Label:   "文件",
			Submenu: fileSubmenu,
		},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleViewMenu},
		{Role: mygo.RoleWindowMenu},
	})
}

func setupAppMenu(win *mygo.Window) {
	menu := buildAppMenu(win)
	mygo.App.SetMenu(menu)
	win.SetAutoHideMenuBar(true)
}

func resolveConfig(opts Options) data.Config {
	clientCfg, err := config.LoadClient(opts.ConfigFile)
	if err != nil {
		log.Printf("[CLIENT] load config %s: %v, falling back to defaults", opts.ConfigFile, err)
		clientCfg = config.DefaultClient()
	}

	// CLI flags override YAML config
	if opts.Driver != "" {
		clientCfg.Driver = strings.ToLower(strings.TrimSpace(opts.Driver))
	}
	if opts.APIURL != "" {
		clientCfg.Driver = "remote"
		clientCfg.APIURL = strings.TrimRight(opts.APIURL, "/")
	}
	if opts.SQLitePath != "" {
		clientCfg.SQLitePath = opts.SQLitePath
	}
	if opts.Token != "" {
		clientCfg.Token = opts.Token
	}

	cfg := data.Config{
		Driver: clientCfg.Driver,
		Token:  clientCfg.Token,
	}

	switch cfg.Driver {
	case "remote", "http", "api":
		cfg.Driver = "remote"
		cfg.DSN = clientCfg.APIURL
		if cfg.DSN == "" {
			cfg.DSN = "http://127.0.0.1:8099"
		}
	default:
		cfg.Driver = "sqlite"
		cfg.DSN = clientCfg.SQLitePath
		if cfg.DSN == "" {
			dir, err := mygo.App.Path(mygo.PathUserData)
			if err != nil || dir == "" {
				dir = "."
			}
			cfg.DSN = filepath.Join(dir, "school.db")
		}
	}
	return cfg
}
