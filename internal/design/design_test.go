package design

import "testing"

// TestMoney checks the format every amount goes through, including the
// thousands and the sign of a 消课.
func TestMoney(t *testing.T) {
	cases := map[int64]string{
		0:         "¥0.00",
		5:         "¥0.05",
		123456:    "¥1,234.56",
		100000000: "¥1,000,000.00",
		-500:      "-¥5.00",
	}
	for cents, want := range cases {
		if got := Money(cents); got != want {
			t.Errorf("Money(%d) = %q, want %q", cents, got, want)
		}
	}
}

// TestCentsRoundTrip checks that a yuan amount the form shows survives the
// trip back to cents, which is what a 充值 depends on.
func TestCentsRoundTrip(t *testing.T) {
	for _, yuan := range []float64{0, 0.01, 99.99, 1280, 48000} {
		cents := Cents(yuan)
		if got := Yuan(cents); got != yuan {
			t.Errorf("Yuan(Cents(%v)) = %v", yuan, got)
		}
	}
}
