package features

import (
	"context"
	"errors"
	"strings"
	"time"

	"mygo-app/internal/design"
	"mygo-app/internal/domain"

	"github.com/egoist/mygo/ui"
)

// Login is the door of the app. It draws before any page exists, so no
// query runs for someone who has not signed in yet, and the shell is built
// only once the store has answered.
type Login struct {
	repo domain.Repo
	win  design.Updater

	// onSignedIn runs on the main thread with the user the store returned.
	onSignedIn func(domain.User)

	username string
	password string
	busy     bool
	err      error
}

// NewLogin builds the sign-in screen. onSignedIn is called once per
// successful login, from the frame that receives it.
func NewLogin(repo domain.Repo, win design.Updater, onSignedIn func(domain.User)) *Login {
	return &Login{repo: repo, win: win, onSignedIn: onSignedIn}
}

func (l *Login) submit() {
	if l.busy {
		return
	}
	username := strings.TrimSpace(l.username)
	if username == "" || l.password == "" {
		l.err = errors.New("请输入用户名和密码")
		return
	}
	l.busy, l.err = true, nil
	win, repo, password := l.win, l.repo, l.password
	go func() {
		user, err := repo.Auth().Login(context.Background(), username, password)
		win.Update(func() {
			l.busy = false
			l.password = ""
			if err != nil {
				l.err = err
				return
			}
			l.err = nil
			l.onSignedIn(user)
		})
	}()
}

// View draws the sign-in card in the middle of the window. The card rises
// and settles as it appears, as the window opens.
func (l *Login) View(c *ui.Context) {
	t := design.Theme(c)
	ui.Column(c).Fill().Center().Background(design.LookFor(c).Page).Children(func() {
		ui.Box(c).Key("login-card").Transition(ui.ElementTransition{
			Duration: 260 * time.Millisecond,
			Enter:    &ui.Motion{Y: 10, Scale: 0.97, Opacity: 0},
		}).Children(func() {
			design.Card(c, func() {
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, "启明培训").FontSize(22).Bold()
					ui.Text(c, "管理后台").FontSize(12).TextColor(t.TextMuted)
				})
				ui.Field(c, "用户名", func() {
					if ui.TextInput(c, &l.username).AutoFocus().Submitted() {
						l.submit()
					}
				})
				ui.Field(c, "密码", func() {
					if ui.TextInput(c, &l.password).Password().Submitted() {
						l.submit()
					}
				})
				design.FormError(c, errText(l.err))
				label := "登录"
				if l.busy {
					label = "登录中…"
				}
				if ui.PrimaryButton(c, label).Fill().Height(32).Disabled(l.busy).Clicked() {
					l.submit()
				}
				ui.Text(c, "首次使用：admin / admin123（上线前请改掉）").
					FontSize(11).TextColor(t.TextMuted)
			}).Width(340)
		})
	})
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
