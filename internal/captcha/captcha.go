package captcha

import (
	"context"
	"encoding/base64"
	"image/color"
	"strings"
	"time"

	"github.com/mojocn/base64Captcha"
)

// Item holds the identifier and base64-encoded image string of a generated captcha.
type Item struct {
	ID    string `json:"id"`
	Image string `json:"image"`
}

// Service defines operations for generating and validating captchas.
type Service interface {
	Generate() (Item, error)
	Verify(id, answer string) bool
}

// Provider represents any source capable of providing a captcha challenge.
type Provider interface {
	GetCaptcha(ctx context.Context) (Item, error)
}

// Credentials holds captcha challenge ID and user's calculated answer.
type Credentials struct {
	ID   string
	Code string
}

type contextKey struct{}

// WithCredentials returns a context containing captcha credentials.
func WithCredentials(ctx context.Context, id, code string) context.Context {
	return context.WithValue(ctx, contextKey{}, Credentials{ID: id, Code: code})
}

// CredentialsFromContext retrieves captcha credentials from context if available.
func CredentialsFromContext(ctx context.Context) (Credentials, bool) {
	cred, ok := ctx.Value(contextKey{}).(Credentials)
	return cred, ok
}

type mathCaptchaService struct {
	captcha *base64Captcha.Captcha
}

// NewMathCaptchaService creates a math-based graphic captcha service.
// Generates simple arithmetic questions (e.g. 5 + 3 = ?).
func NewMathCaptchaService() Service {
	// Driver parameters: height 38, width 110, noiseCount 0, showLineOptions 0
	// Background: soft light gray
	bgColor := &color.RGBA{R: 245, G: 247, B: 250, A: 255}
	driver := base64Captcha.NewDriverMath(38, 110, 0, 0, bgColor, nil, []string{"wqy-microhei.ttc"})
	store := base64Captcha.NewMemoryStore(10240, 5*time.Minute)
	c := base64Captcha.NewCaptcha(driver, store)
	return &mathCaptchaService{captcha: c}
}

func (s *mathCaptchaService) Generate() (Item, error) {
	id, b64s, _, err := s.captcha.Generate()
	if err != nil {
		return Item{}, err
	}
	return Item{ID: id, Image: b64s}, nil
}

func (s *mathCaptchaService) Verify(id, answer string) bool {
	id = strings.TrimSpace(id)
	answer = strings.TrimSpace(answer)
	if id == "" || answer == "" {
		return false
	}
	return s.captcha.Verify(id, answer, true)
}

func (s *mathCaptchaService) GetCaptcha(ctx context.Context) (Item, error) {
	return s.Generate()
}

// ExtractPNGBytes extracts the raw PNG byte slice from a data URI or base64 string.
func ExtractPNGBytes(b64s string) ([]byte, error) {
	data := b64s
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}
	return base64.StdEncoding.DecodeString(data)
}
