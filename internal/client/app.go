// Package client implements the desktop GUI application.
package client

import (
	"context"
	"log"
	"os"
	"path/filepath"

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
			r.win.Invalidate()
		}
	}
}

// Run launches the desktop GUI client application.
func Run() {
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

		repo, err := data.Open(context.Background(), config())
		if err != nil {
			r.openErr = err
			log.Printf("open store: %v", err)
		} else {
			r.repo = repo
			r.login = features.NewLogin(repo, r.win, r.signedIn)
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

func config() data.Config {
	cfg := data.Config{
		Driver: os.Getenv("SCHOOL_DRIVER"),
		Token:  os.Getenv("SCHOOL_TOKEN"),
		DSN:    os.Getenv("SCHOOL_DB"),
	}
	if api := os.Getenv("SCHOOL_API"); api != "" {
		cfg.Driver, cfg.DSN = "remote", api
	}
	if cfg.Driver == "" || cfg.Driver == "sqlite" {
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
