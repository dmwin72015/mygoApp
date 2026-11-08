package tokens_test

import (
	"testing"

	"mygo-app/internal/design/tokens"
)

func TestTokenDerivation(t *testing.T) {
	light := tokens.DeriveTokens(false, tokens.DefaultBrandPrimary)
	if light.Dark {
		t.Fatal("expected light tokens to have Dark=false")
	}
	if light.BrandPrimary == light.BrandHover {
		t.Fatal("expected BrandHover to be mathematically distinct from BrandPrimary")
	}

	dark := tokens.DeriveTokens(true, tokens.DefaultBrandPrimary)
	if !dark.Dark {
		t.Fatal("expected dark tokens to have Dark=true")
	}
	if dark.BrandPrimary == dark.BrandHover {
		t.Fatal("expected BrandHover in dark mode to be mathematically distinct")
	}
}

func TestMetricsConstants(t *testing.T) {
	if tokens.RadiusMd != 8 {
		t.Fatalf("expected RadiusMd to be 8, got %v", tokens.RadiusMd)
	}
	if tokens.ControlHeight != 34 {
		t.Fatalf("expected ControlHeight to be 34, got %v", tokens.ControlHeight)
	}
}
