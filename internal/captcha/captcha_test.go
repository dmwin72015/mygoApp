package captcha_test

import (
	"testing"

	"mygo-app/internal/captcha"

	"github.com/egoist/mygo/ui"
)

func TestMathCaptchaService(t *testing.T) {
	svc := captcha.NewMathCaptchaService()

	item, err := svc.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if item.ID == "" {
		t.Fatalf("expected non-empty captcha ID")
	}
	if len(item.Image) == 0 {
		t.Fatalf("expected non-empty captcha image string")
	}

	// Test decoding raw PNG bytes and verifying ui.DecodeImage compatibility
	rawBytes, err := captcha.ExtractPNGBytes(item.Image)
	if err != nil {
		t.Fatalf("ExtractPNGBytes failed: %v", err)
	}
	if len(rawBytes) == 0 {
		t.Fatalf("expected non-empty raw PNG bytes")
	}

	img, err := ui.DecodeImage(rawBytes)
	if err != nil {
		t.Fatalf("ui.DecodeImage failed: %v", err)
	}
	if img == nil {
		t.Fatalf("expected valid ui.ImageSource")
	}

	// Verify wrong answer fails
	if svc.Verify(item.ID, "wrong_answer") {
		t.Fatalf("expected wrong answer to fail verification")
	}

	// Verify empty inputs fail
	if svc.Verify("", "") {
		t.Fatalf("expected empty inputs to fail verification")
	}
}
