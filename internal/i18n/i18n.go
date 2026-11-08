package i18n

import (
	"fmt"
	"strings"
	"sync"
)

// Supported language codes.
const (
	LangZH = "zh-CN"
	LangEN = "en-US"
)

var (
	mu          sync.RWMutex
	currentLang = LangZH

	// OnLanguageChange is invoked when the client switches language.
	OnLanguageChange func(lang string)
)

// Current returns the active client language.
func Current() string {
	mu.RLock()
	defer mu.RUnlock()
	return currentLang
}

// SetLanguage sets the active language and triggers OnLanguageChange.
func SetLanguage(lang string) {
	norm := Normalize(lang)
	mu.Lock()
	if currentLang == norm {
		mu.Unlock()
		return
	}
	currentLang = norm
	callback := OnLanguageChange
	mu.Unlock()

	if callback != nil {
		callback(norm)
	}
}

// Normalize parses and normalizes language headers/strings into supported tags.
func Normalize(lang string) string {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return LangZH
	}
	// Extract primary language tag if multiple provided (e.g. "en-US,en;q=0.9")
	if idx := strings.IndexByte(lang, ','); idx != -1 {
		lang = strings.TrimSpace(lang[:idx])
	}
	if idx := strings.IndexByte(lang, ';'); idx != -1 {
		lang = strings.TrimSpace(lang[:idx])
	}
	lang = strings.ReplaceAll(lang, "_", "-")
	lower := strings.ToLower(lang)

	if strings.HasPrefix(lower, "en") {
		return LangEN
	}
	if strings.HasPrefix(lower, "zh") {
		return LangZH
	}
	return LangZH
}

// T translates key into the currently active client language.
func T(key string, args ...any) string {
	return Tr(Current(), key, args...)
}

// Tr translates key into the specified language.
func Tr(lang, key string, args ...any) string {
	target := Normalize(lang)

	dict, ok := translations[target]
	if !ok {
		dict = translations[LangZH]
	}

	text, ok := dict[key]
	if !ok {
		// Fallback to Chinese dictionary
		if target != LangZH {
			if fb, exists := translations[LangZH][key]; exists {
				text = fb
			} else {
				text = key
			}
		} else {
			text = key
		}
	}

	if len(args) > 0 {
		return fmt.Sprintf(text, args...)
	}
	return text
}

// Translations dictionary
var translations = map[string]map[string]string{
	LangZH: {
		// App & Shell
		"app.title":              "启明培训 · 管理后台",
		"app.loading":            "正在启动…",
		"app.load_data":          "数据",
		"app.err_open_store":     "无法打开数据存储",
		"app.err_hint":           "检查 SCHOOL_DRIVER、SCHOOL_DB、SCHOOL_API 后重开。",
		"app.quit.title":         "确定要退出管理后台吗？",
		"app.quit.desc":          "未保存的编辑将会丢失。",
		"btn.cancel":             "取消",
		"btn.quit":               "退出",
		"btn.save":               "保存",
		"btn.delete":             "删除",
		"btn.edit":               "编辑",
		"btn.add":                "新增",
		"btn.search":             "搜索",
		"btn.refresh":            "刷新",
		"btn.close":              "关闭",
		"btn.confirm":            "确定",
		"btn.signout":            "退出登录",
		"btn.updating":           "处理中...",

		// Navigation
		"nav.overview":           "概览",
		"nav.students":           "学员管理",
		"nav.teachers":           "教师管理",
		"nav.classrooms":         "教室管理",
		"nav.courses":            "课程管理",
		"nav.lessons":            "排课计划",
		"nav.ledger":             "资金账本",
		"nav.users":              "账号管理",
		"nav.settings":           "系统设置",

		// Roles
		"role.admin":             "系统管理员",
		"role.teacher":           "任课教师",
		"role.staff":             "教务运营",

		// Settings page
		"settings.title":         "设置",
		"settings.subtitle":      "语言、主题与运行环境",
		"settings.lang_card":     "语言设置 (Language)",
		"settings.lang_desc":     "选择应用界面的显示语言，即时生效并自动保存。",
		"settings.appearance":    "外观主题",
		"settings.theme_auto":    "跟随系统",
		"settings.theme_light":   "浅色模式",
		"settings.theme_dark":    "深色模式",
		"settings.theme_desc":    "选「跟随系统」时随系统的浅色 / 深色切换。选择会存进数据库，重启后仍在。",
		"settings.accent":        "强调色",
		"settings.accent_val":    "靛蓝 #4f46e5 / #6366f1",
		"settings.datasource":    "数据存储",
		"settings.source_curr":   "当前来源",
		"settings.source_sqlite": "本地 SQLite 数据库",
		"settings.source_remote": "远程 REST API 服务",
		"settings.pwd_card":      "修改密码",
		"settings.pwd_old":       "原密码",
		"settings.pwd_old_ph":    "当前登录密码",
		"settings.pwd_new":       "新密码",
		"settings.pwd_new_ph":    "至少 6 位",
		"settings.pwd_confirm":   "确认新密码",
		"settings.pwd_conf_ph":   "再次输入新密码",
		"settings.pwd_btn":       "更新密码",
		"settings.pwd_success":   "密码修改成功",
		"settings.pwd_err_old":   "请输入原密码",
		"settings.pwd_err_len":   "新密码至少 6 位",
		"settings.pwd_err_match": "两次输入的新密码不一致",
		"settings.pwd_err_wrong": "原密码不正确",

		// Server errors & status
		"err.bad_credentials":    "用户名或密码不正确",
		"err.bad_captcha":        "验证码错误或已过期",
		"err.captcha_req":        "请输入验证码计算结果",
		"err.unauthorized":       "未提供认证令牌或登录已失效",
		"err.forbidden":          "无权执行该操作",
		"err.not_found":          "记录未找到",
		"err.invalid_req":        "请求参数格式不正确",
		"err.username_req":       "用户名不能为空",
		"err.user_exists":        "用户名已存在",
		"err.password_min":       "密码至少 6 位",
		"err.at_least_one_admin": "至少要保留一个管理员",
		"err.name_required":      "姓名不能为空",
		"err.time_required":      "上课时间不能为空",
		"err.amount_positive":    "金额必须大于 0",
		"err.no_perm_other_user": "无权修改其他账号",
		"err.cannot_delete_self": "不能删除当前登录账号",
		"msg.login_success":      "登录成功",
		"msg.pwd_reset_success":  "密码重置成功，请使用新密码登录",

		// Registration & Auth UI
		"auth.login_title":       "登录管理后台",
		"auth.captcha":           "验证码",
		"auth.captcha_ph":        "计算结果",
		"auth.captcha_refresh":   "点击图片刷新验证码",
		"auth.signup_title":      "创建新账号",
		"auth.signup_sub":        "注册教务或教师账号，即时使用",
		"auth.display_name":      "姓名 / 显示名",
		"auth.display_name_ph":   "例如：张老师（选填）",
		"auth.signup_btn":        "立即注册",
		"auth.signing_up":        "正在注册…",
		"auth.no_account":        "还没有账号？立即注册",
		"auth.have_account":      "已有账号？返回登录",
		"auth.confirm_pwd":       "确认密码",
		"auth.confirm_pwd_ph":    "再次输入密码",
		"auth.pwd_mismatch":      "两次输入的密码不一致",
		"msg.register_success":   "注册成功并已自动登录",
		"auth.signout_title":     "退出登录确认",
		"auth.signout_desc":      "确定要退出当前登录账号吗？退出后需重新输入密码登录。",
	},

	LangEN: {
		// App & Shell
		"app.title":              "Qiming Education · Admin",
		"app.loading":            "Starting…",
		"app.load_data":          "data",
		"app.err_open_store":     "Failed to open data store",
		"app.err_hint":           "Check SCHOOL_DRIVER, SCHOOL_DB, SCHOOL_API and restart.",
		"app.quit.title":         "Are you sure you want to quit?",
		"app.quit.desc":          "Unsaved changes will be lost.",
		"btn.cancel":             "Cancel",
		"btn.quit":               "Quit",
		"btn.save":               "Save",
		"btn.delete":             "Delete",
		"btn.edit":               "Edit",
		"btn.add":                "Add",
		"btn.search":             "Search",
		"btn.refresh":            "Refresh",
		"btn.close":              "Close",
		"btn.confirm":            "OK",
		"btn.signout":            "Sign Out",
		"btn.updating":           "Processing...",

		// Navigation
		"nav.overview":           "Overview",
		"nav.students":           "Students",
		"nav.teachers":           "Teachers",
		"nav.classrooms":         "Classrooms",
		"nav.courses":            "Courses",
		"nav.lessons":            "Lessons",
		"nav.ledger":             "Billing Ledger",
		"nav.users":              "User Accounts",
		"nav.settings":           "Settings",

		// Roles
		"role.admin":             "Administrator",
		"role.teacher":           "Teacher",
		"role.staff":             "Staff",

		// Settings page
		"settings.title":         "Settings",
		"settings.subtitle":      "Language, theme and environment",
		"settings.lang_card":     "Language",
		"settings.lang_desc":     "Select display language. Changes take effect immediately and are saved.",
		"settings.appearance":    "Appearance & Theme",
		"settings.theme_auto":    "System",
		"settings.theme_light":   "Light",
		"settings.theme_dark":    "Dark",
		"settings.theme_desc":    "Follows OS light/dark mode when set to System. Persisted across restarts.",
		"settings.accent":        "Accent Color",
		"settings.accent_val":    "Indigo #4f46e5 / #6366f1",
		"settings.datasource":    "Data Source",
		"settings.source_curr":   "Current Source",
		"settings.source_sqlite": "Local SQLite Database",
		"settings.source_remote": "Remote REST API Service",
		"settings.pwd_card":      "Change Password",
		"settings.pwd_old":       "Current Password",
		"settings.pwd_old_ph":    "Enter current password",
		"settings.pwd_new":       "New Password",
		"settings.pwd_new_ph":    "At least 6 characters",
		"settings.pwd_confirm":   "Confirm Password",
		"settings.pwd_conf_ph":   "Re-enter new password",
		"settings.pwd_btn":       "Update Password",
		"settings.pwd_success":   "Password updated successfully",
		"settings.pwd_err_old":   "Please enter current password",
		"settings.pwd_err_len":   "New password must be at least 6 characters",
		"settings.pwd_err_match": "Passwords do not match",
		"settings.pwd_err_wrong": "Incorrect current password",

		// Server errors & status
		"err.bad_credentials":    "Invalid username or password",
		"err.bad_captcha":        "Invalid or expired verification code",
		"err.captcha_req":        "Please enter the captcha result",
		"err.unauthorized":       "Authentication token missing or session expired",
		"err.forbidden":          "Permission denied",
		"err.not_found":          "Record not found",
		"err.invalid_req":        "Invalid request parameters",
		"err.username_req":       "Username is required",
		"err.user_exists":        "Username already exists",
		"err.password_min":       "Password must be at least 6 characters",
		"err.at_least_one_admin": "At least one administrator must be preserved",
		"err.name_required":      "Name is required",
		"err.time_required":      "Lesson start time is required",
		"err.amount_positive":    "Amount must be greater than zero",
		"err.no_perm_other_user": "Permission denied to modify other accounts",
		"err.cannot_delete_self": "Cannot delete currently signed-in account",
		"msg.login_success":      "Login successful",
		"msg.pwd_reset_success":  "Password reset successfully, please sign in with new password",

		// Registration & Auth UI
		"auth.login_title":       "Sign In to Admin",
		"auth.captcha":           "Captcha",
		"auth.captcha_ph":        "Answer",
		"auth.captcha_refresh":   "Click image to refresh captcha",
		"auth.signup_title":      "Create an Account",
		"auth.signup_sub":        "Register a staff or teacher account",
		"auth.display_name":      "Display Name",
		"auth.display_name_ph":   "e.g. Alex Zhang (optional)",
		"auth.signup_btn":        "Register Now",
		"auth.signing_up":        "Registering…",
		"auth.no_account":        "Don't have an account? Sign Up",
		"auth.have_account":      "Already have an account? Sign In",
		"auth.confirm_pwd":       "Confirm Password",
		"auth.confirm_pwd_ph":    "Re-enter password",
		"auth.pwd_mismatch":      "Passwords do not match",
		"msg.register_success":   "Registration successful and signed in",
		"auth.signout_title":     "Sign Out Confirmation",
		"auth.signout_desc":      "Are you sure you want to sign out? You will need to log in again.",
	},
}
