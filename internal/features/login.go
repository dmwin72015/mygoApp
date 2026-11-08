package features

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"time"

	"mygo-app/internal/captcha"
	"mygo-app/internal/design"
	"mygo-app/internal/design/tokens"
	"mygo-app/internal/domain"
	"mygo-app/internal/i18n"

	"github.com/egoist/mygo/ui"
)

type loginMode int

const (
	modeSignIn loginMode = iota
	modeForgot
	modeSignUp
)

// Login is the door of the app. It draws before any page exists, so no
// query runs for someone who has not signed in yet, and the shell is built
// only once the store has answered.
type Login struct {
	repo domain.Repo
	win  design.Updater

	// onSignedIn runs on the main thread with the user the store returned.
	onSignedIn func(domain.User)

	// OnOpenSettings opens the standalone settings window if configured.
	OnOpenSettings func()

	mode loginMode

	// Sign In fields
	username       string
	password       string
	captchaID      string
	captchaCode    string
	captchaImg     ui.ImageSource
	captchaLoading bool
	captchaLoaded  bool
	localCaptcha   captcha.Service

	// Forgot Password fields
	forgotUser      string
	forgotKey       string
	newPassword     string
	confirmPassword string
	successMsg      string

	// Sign Up fields
	regUsername        string
	regDisplayName     string
	regPassword        string
	regConfirmPassword string

	busy bool
	err  error
}

// NewLogin builds the sign-in screen. onSignedIn is called once per
// successful login, from the frame that receives it.
func NewLogin(repo domain.Repo, win design.Updater, onSignedIn func(domain.User)) *Login {
	return &Login{
		repo:         repo,
		win:          win,
		onSignedIn:   onSignedIn,
		localCaptcha: captcha.NewMathCaptchaService(),
	}
}

func (l *Login) refreshCaptcha() {
	if l.captchaLoading {
		return
	}
	l.captchaLoading = true
	l.captchaCode = ""

	// If running with local service, generate synchronously for instant rendering without race conditions
	if _, isRemote := l.repo.(captcha.Provider); !isRemote && l.localCaptcha != nil {
		item, err := l.localCaptcha.Generate()
		if err == nil {
			raw, decErr := captcha.ExtractPNGBytes(item.Image)
			if decErr == nil {
				l.captchaImg, _ = ui.DecodeImage(raw)
			}
			l.captchaID = item.ID
			l.captchaLoaded = true
		}
		l.captchaLoading = false
		return
	}

	win, repo := l.win, l.repo
	go func() {
		var item captcha.Item
		var err error
		if provider, ok := repo.(captcha.Provider); ok {
			item, err = provider.GetCaptcha(context.Background())
		}
		if err != nil {
			if win != nil {
				win.Update(func() {
					l.captchaLoading = false
					l.err = err
				})
			} else {
				l.captchaLoading = false
				l.err = err
			}
			return
		}
		raw, decErr := captcha.ExtractPNGBytes(item.Image)
		var imgSource ui.ImageSource
		if decErr == nil {
			imgSource, _ = ui.DecodeImage(raw)
		}
		if win != nil {
			win.Update(func() {
				l.captchaLoading = false
				l.captchaLoaded = true
				l.captchaID = item.ID
				l.captchaImg = imgSource
			})
		} else {
			l.captchaLoading = false
			l.captchaLoaded = true
			l.captchaID = item.ID
			l.captchaImg = imgSource
		}
	}()
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
	captchaCode := strings.TrimSpace(l.captchaCode)
	if l.captchaID != "" && captchaCode == "" {
		l.err = errors.New(i18n.T("err.captcha_req"))
		return
	}

	// In standalone/local mode without remote API, verify captcha with local service
	if _, isRemote := l.repo.(captcha.Provider); !isRemote && l.localCaptcha != nil && l.captchaID != "" {
		if !l.localCaptcha.Verify(l.captchaID, captchaCode) {
			l.err = errors.New(i18n.T("err.bad_captcha"))
			l.refreshCaptcha()
			return
		}
	}

	l.busy, l.err = true, nil
	win, repo, password, captchaID := l.win, l.repo, l.password, l.captchaID
	go func() {
		ctx := captcha.WithCredentials(context.Background(), captchaID, captchaCode)
		user, err := repo.Auth().Login(ctx, username, password)
		if win != nil {
			win.Update(func() {
				l.busy = false
				l.password = ""
				if err != nil {
					l.err = err
					l.refreshCaptcha()
					return
				}
				l.err = nil
				l.onSignedIn(user)
			})
		} else {
			l.busy = false
			l.password = ""
			if err != nil {
				l.err = err
				l.refreshCaptcha()
				return
			}
			l.err = nil
			l.onSignedIn(user)
		}
	}()
}

// SignInForTest allows headless tests to trigger the sign in flow directly.
func (l *Login) SignInForTest(username, password, captchaCode string) {
	l.username = username
	l.password = password
	l.captchaCode = captchaCode
	l.submit()
}

// SetCaptchaForTest injects a pre-set captcha ID and image for testing.
func (l *Login) SetCaptchaForTest(id, code string, img ui.ImageSource) {
	l.captchaID = id
	l.captchaCode = code
	l.captchaImg = img
	l.captchaLoaded = true
}

// CaptchaID returns the current captcha ID.
func (l *Login) CaptchaID() string {
	return l.captchaID
}

// SetLocalCaptchaService configures the local captcha service (useful in tests).
func (l *Login) SetLocalCaptchaService(svc captcha.Service) {
	l.localCaptcha = svc
}

// FormError returns the current error message on the login card, if any.
func (l *Login) FormError() string {
	if l.err != nil {
		return l.err.Error()
	}
	return ""
}

func (l *Login) submitForgot() {
	if l.busy {
		return
	}
	username := strings.TrimSpace(l.forgotUser)
	key := strings.TrimSpace(l.forgotKey)
	newPass := strings.TrimSpace(l.newPassword)
	confirmPass := strings.TrimSpace(l.confirmPassword)

	if username == "" {
		l.err = errors.New("请输入用户名")
		return
	}
	if key == "" {
		l.err = errors.New("请输入安全恢复密钥")
		return
	}
	if len(newPass) < 6 {
		l.err = errors.New("新密码长度不能少于 6 位")
		return
	}
	if newPass != confirmPass {
		l.err = errors.New("两次输入的新密码不一致")
		return
	}

	l.busy, l.err = true, nil
	win, repo := l.win, l.repo
	go func() {
		err := repo.Auth().ResetPassword(context.Background(), username, key, newPass)
		win.Update(func() {
			l.busy = false
			if err != nil {
				l.err = err
				return
			}
			l.err = nil
			l.successMsg = "密码重置成功！请使用新密码登录。"
			l.username = username
			l.password = ""
			l.forgotKey = ""
			l.newPassword = ""
			l.confirmPassword = ""
		})
	}()
}

// ResetPasswordForTest allows headless tests to trigger the forgot password flow directly.
func (l *Login) ResetPasswordForTest(username, key, newPass, confirmPass string) {
	l.forgotUser = username
	l.forgotKey = key
	l.newPassword = newPass
	l.confirmPassword = confirmPass
	l.submitForgot()
}

func (l *Login) submitSignUp() {
	if l.busy {
		return
	}
	username := strings.TrimSpace(l.regUsername)
	displayName := strings.TrimSpace(l.regDisplayName)
	password := l.regPassword
	confirmPassword := l.regConfirmPassword

	if username == "" {
		l.err = errors.New(i18n.T("err.username_req"))
		return
	}
	if len(password) < 6 {
		l.err = errors.New(i18n.T("err.password_min"))
		return
	}
	if password != confirmPassword {
		l.err = errors.New(i18n.T("auth.pwd_mismatch"))
		return
	}

	l.busy, l.err = true, nil
	win, repo := l.win, l.repo
	go func() {
		user, err := repo.Auth().Register(context.Background(), username, password, displayName)
		win.Update(func() {
			l.busy = false
			if err != nil {
				l.err = err
				return
			}
			l.err = nil
			l.regPassword = ""
			l.regConfirmPassword = ""
			l.onSignedIn(user)
		})
	}()
}

// SignUpForTest allows headless tests to trigger the registration flow directly.
func (l *Login) SignUpForTest(username, password, confirmPassword, displayName string) {
	l.regUsername = username
	l.regPassword = password
	l.regConfirmPassword = confirmPassword
	l.regDisplayName = displayName
	l.submitSignUp()
}

// View draws the sign-in card in the middle of the window. The card rises
// and settles as it appears, as the window opens.
func (l *Login) View(c *ui.Context) {
	if !l.captchaLoaded && !l.captchaLoading {
		l.refreshCaptcha()
	}
	t := design.Theme(c)
	tok := tokens.Get(c)

	ui.Column(c).Fill().Center().Background(design.LookFor(c).Page).Children(func() {
		ui.Box(c).Key("login-card").Transition(ui.ElementTransition{
			Duration: 260 * time.Millisecond,
			Enter:    &ui.Motion{Y: 10, Scale: 0.97, Opacity: 0},
		}).Children(func() {
			switch l.mode {
			case modeForgot:
				l.renderForgotCard(c, t, tok)
			case modeSignUp:
				l.renderSignUpCard(c, t, tok)
			default:
				l.renderSignInCard(c, t, tok)
			}
		})
	})
}

func (l *Login) renderSignInCard(c *ui.Context, t *ui.Theme, tok *tokens.SemanticTokens) {
	design.Card(c, func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(3).Children(func() {
				ui.Text(c, "启明培训").FontSize(22).Bold()
				ui.Text(c, "管理后台").FontSize(12).TextColor(t.TextMuted)
			})
			if runtime.GOOS != "darwin" && l.OnOpenSettings != nil {
				ui.Spacer(c)
				if ui.Button(c, "⚙ "+i18n.T("settings.title")).Height(28).Clicked() {
					l.OnOpenSettings()
				}
			}
		})
		if l.successMsg != "" {
			ui.Row(c).Padding(6, 10).Radius(tokens.RadiusSm).
				Background(tok.Success.Alpha(0.12)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "✓ "+l.successMsg).FontSize(12).TextColor(tok.Success).Bold()
			})
		}
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
		ui.Field(c, i18n.T("auth.captcha"), func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if ui.TextInput(c, &l.captchaCode).Placeholder(i18n.T("auth.captcha_ph")).Width(160).Submitted() {
					l.submit()
				}
				if l.captchaImg != nil {
					if ui.Image(c, l.captchaImg).
						Width(110).
						Height(36).
						Radius(4).
						Cursor(ui.CursorPointer).
						Tooltip(i18n.T("auth.captcha_refresh")).
						Clicked() {
						l.refreshCaptcha()
					}
				} else {
					label := "获取验证码"
					if l.captchaLoading {
						label = "加载中…"
					}
					if ui.Button(c, label).Height(36).Disabled(l.captchaLoading).Clicked() {
						l.refreshCaptcha()
					}
				}
			})
		})
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			if ui.Text(c, i18n.T("auth.no_account")).
				FontSize(12).
				TextColor(tok.BrandPrimary).
				Cursor(ui.CursorPointer).
				Focusable().
				Clicked() {
				l.mode = modeSignUp
				l.err = nil
				l.successMsg = ""
				if l.username != "" {
					l.regUsername = l.username
				}
			}
			if ui.Text(c, "忘记密码？").
				FontSize(12).
				TextColor(tok.BrandPrimary).
				Cursor(ui.CursorPointer).
				Focusable().
				Clicked() {
				l.mode = modeForgot
				l.err = nil
				l.successMsg = ""
				if l.username != "" {
					l.forgotUser = l.username
				}
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
		if l.repo != nil {
			ui.Text(c, "数据源: "+l.repo.Source()).
				FontSize(11).TextColor(t.TextMuted)
		}
	}).Width(340)
}

func (l *Login) renderSignUpCard(c *ui.Context, t *ui.Theme, tok *tokens.SemanticTokens) {
	design.Card(c, func() {
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, i18n.T("auth.signup_title")).FontSize(20).Bold()
			ui.Text(c, i18n.T("auth.signup_sub")).FontSize(12).TextColor(t.TextMuted)
		})
		ui.Field(c, "用户名", func() {
			if ui.TextInput(c, &l.regUsername).Placeholder("用于登录的账号").AutoFocus().Submitted() {
				l.submitSignUp()
			}
		})
		ui.Field(c, i18n.T("auth.display_name"), func() {
			if ui.TextInput(c, &l.regDisplayName).Placeholder(i18n.T("auth.display_name_ph")).Submitted() {
				l.submitSignUp()
			}
		})
		ui.Field(c, "密码", func() {
			if ui.TextInput(c, &l.regPassword).Password().Placeholder("不少于 6 位").Submitted() {
				l.submitSignUp()
			}
		})
		ui.Field(c, i18n.T("auth.confirm_pwd"), func() {
			if ui.TextInput(c, &l.regConfirmPassword).Password().Placeholder(i18n.T("auth.confirm_pwd_ph")).Submitted() {
				l.submitSignUp()
			}
		})
		design.FormError(c, errText(l.err))
		btnLabel := i18n.T("auth.signup_btn")
		if l.busy {
			btnLabel = i18n.T("auth.signing_up")
		}
		if ui.PrimaryButton(c, btnLabel).Fill().Height(32).Disabled(l.busy).Clicked() {
			l.submitSignUp()
		}
		if ui.Button(c, i18n.T("auth.have_account")).Fill().Height(30).Disabled(l.busy).Clicked() {
			l.mode = modeSignIn
			l.err = nil
			l.successMsg = ""
			if l.regUsername != "" {
				l.username = l.regUsername
			}
		}
	}).Width(360)
}

func (l *Login) renderForgotCard(c *ui.Context, t *ui.Theme, tok *tokens.SemanticTokens) {
	design.Card(c, func() {
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, "找回密码").FontSize(20).Bold()
			ui.Text(c, "使用安全恢复密钥重置个人或管理员密码").FontSize(12).TextColor(t.TextMuted)
		})
		if l.successMsg != "" {
			ui.Column(c).Gap(10).Children(func() {
				ui.Row(c).Padding(10, 12).Radius(tokens.RadiusSm).
					Background(tok.Success.Alpha(0.12)).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "✓ "+l.successMsg).FontSize(13).TextColor(tok.Success).Bold()
				})
				if ui.PrimaryButton(c, "返回登录").Fill().Height(32).Clicked() {
					l.mode = modeSignIn
					l.err = nil
				}
			})
			return
		}
		ui.Field(c, "账号用户名", func() {
			if ui.TextInput(c, &l.forgotUser).Placeholder("输入要找回的用户名").AutoFocus().Submitted() {
				l.submitForgot()
			}
		})
		ui.Field(c, "安全恢复密钥", func() {
			if ui.TextInput(c, &l.forgotKey).Password().Placeholder("初始密钥或系统配置密钥").Submitted() {
				l.submitForgot()
			}
		})
		ui.Field(c, "新密码", func() {
			if ui.TextInput(c, &l.newPassword).Password().Placeholder("不少于 6 位").Submitted() {
				l.submitForgot()
			}
		})
		ui.Field(c, "确认新密码", func() {
			if ui.TextInput(c, &l.confirmPassword).Password().Placeholder("再次输入新密码").Submitted() {
				l.submitForgot()
			}
		})
		design.FormError(c, errText(l.err))
		btnLabel := "确认重置密码"
		if l.busy {
			btnLabel = "正在重置…"
		}
		if ui.PrimaryButton(c, btnLabel).Fill().Height(32).Disabled(l.busy).Clicked() {
			l.submitForgot()
		}
		if ui.Button(c, "返回登录").Fill().Height(30).Disabled(l.busy).Clicked() {
			l.mode = modeSignIn
			l.err = nil
			l.successMsg = ""
		}
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, "💡 初始恢复密钥为 qm-recovery-2026（可在设置中自定义）").
				FontSize(11).TextColor(t.TextMuted)
			ui.Text(c, "💡 普通员工亦可联系管理员在后台用户管理中直接重置").
				FontSize(11).TextColor(t.TextMuted)
		})
	}).Width(360)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

