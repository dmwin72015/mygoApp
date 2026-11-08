package remote_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mygo-app/internal/captcha"
	"mygo-app/internal/data/remote"
	"mygo-app/internal/domain"
)

func TestRemoteClientAndRepo(t *testing.T) {
	mux := http.NewServeMux()

	// 1. /auth/captcha
	mux.HandleFunc("/auth/captcha", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		json.NewEncoder(w).Encode(captcha.Item{
			ID:    "test-captcha-id",
			Image: "data:image/png;base64,abc",
		})
	})

	// 2. /auth/login
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["username"] == "admin" && req["password"] == "admin123" {
			json.NewEncoder(w).Encode(map[string]any{
				"token": "test-jwt-token",
				"user": domain.User{
					ID:       1,
					Username: "admin",
					Role:     string(domain.RoleAdmin),
				},
			})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "invalid credentials"})
	})

	// 3. /settings/theme
	mux.HandleFunc("/settings/theme", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]string{"value": "dark"})
			return
		}
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusOK)
			return
		}
	})

	// 4. /stats
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(domain.Stats{
			Students: 10,
			Teachers: 5,
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	repo := remote.Open(ts.URL, "")
	ctx := context.Background()

	// Verify Source
	if repo.Source() != "远程 API · "+ts.URL {
		t.Fatalf("unexpected Source: %s", repo.Source())
	}

	// Verify Captcha
	provider, ok := repo.(captcha.Provider)
	if !ok {
		t.Fatal("expected remote.Repo to implement captcha.Provider")
	}
	item, err := provider.GetCaptcha(ctx)
	if err != nil {
		t.Fatalf("GetCaptcha failed: %v", err)
	}
	if item.ID != "test-captcha-id" {
		t.Fatalf("unexpected captcha ID: %s", item.ID)
	}

	// Verify Auth Login Failure
	_, err = repo.Auth().Login(ctx, "admin", "wrong")
	if err == nil {
		t.Fatal("expected login failure for wrong password")
	}

	// Verify Auth Login Success
	user, err := repo.Auth().Login(ctx, "admin", "admin123")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if user.Username != "admin" {
		t.Fatalf("unexpected user: %+v", user)
	}

	// Verify Settings Get
	val, err := repo.Settings().Get(ctx, "theme")
	if err != nil || val != "dark" {
		t.Fatalf("Settings.Get failed: val=%s, err=%v", val, err)
	}

	// Verify Stats
	stats, err := repo.Stats(ctx)
	if err != nil || stats.Students != 10 {
		t.Fatalf("Stats failed: %+v, err=%v", stats, err)
	}
}
