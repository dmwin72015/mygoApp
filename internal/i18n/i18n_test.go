package i18n

import (
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"zh-CN", LangZH},
		{"zh_CN", LangZH},
		{"zh", LangZH},
		{"en-US", LangEN},
		{"en_US", LangEN},
		{"en", LangEN},
		{"en-GB,en;q=0.9", LangEN},
		{"zh-CN,zh;q=0.9,en;q=0.8", LangZH},
		{"fr-FR", LangZH}, // unknown defaults to zh
		{"", LangZH},
	}

	for _, tt := range tests {
		got := Normalize(tt.input)
		if got != tt.want {
			t.Errorf("Normalize(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestTranslations(t *testing.T) {
	// Chinese translation
	zhTitle := Tr(LangZH, "app.title")
	if zhTitle != "启明培训 · 管理后台" {
		t.Errorf("expected Chinese title, got %q", zhTitle)
	}

	// English translation
	enTitle := Tr(LangEN, "app.title")
	if enTitle != "Qiming Education · Admin" {
		t.Errorf("expected English title, got %q", enTitle)
	}

	// Test fallback for missing key in EN
	fallback := Tr(LangEN, "some_non_existent_key")
	if fallback != "some_non_existent_key" {
		t.Errorf("expected raw key fallback, got %q", fallback)
	}
}

func TestLanguageSwitching(t *testing.T) {
	var notified string
	OnLanguageChange = func(lang string) {
		notified = lang
	}

	SetLanguage("en-US")
	if Current() != LangEN {
		t.Errorf("expected current lang to be EN, got %s", Current())
	}
	if notified != LangEN {
		t.Errorf("expected OnLanguageChange to receive EN, got %s", notified)
	}

	if T("btn.cancel") != "Cancel" {
		t.Errorf("expected Cancel, got %s", T("btn.cancel"))
	}

	SetLanguage("zh-CN")
	if Current() != LangZH {
		t.Errorf("expected current lang to be ZH, got %s", Current())
	}
	if notified != LangZH {
		t.Errorf("expected OnLanguageChange to receive ZH, got %s", notified)
	}
	if T("btn.cancel") != "取消" {
		t.Errorf("expected 取消, got %s", T("btn.cancel"))
	}
}
