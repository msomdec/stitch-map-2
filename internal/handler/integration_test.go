package handler_test

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/handler"
)

func TestIntegration_RegisterLoginDashboardLogout(t *testing.T) {
	auth := newTestAuthService(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't follow redirects automatically
		},
	}

	// 1. Register a new user.
	resp, err := client.PostForm(srv.URL+"/register", url.Values{
		"email":        {"integ@example.com"},
		"display_name": {"Integration User"},
		"password":     {"password123"},
	})
	if err != nil {
		t.Fatalf("POST /register: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("register: expected 303 redirect, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("register: expected redirect to /login, got %s", loc)
	}

	// 2. Login with the new credentials.
	resp, err = client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"integ@example.com"},
		"password": {"password123"},
	})
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: expected 303 redirect, got %d", resp.StatusCode)
	}

	// Verify auth_token cookie was set.
	srvURL, _ := url.Parse(srv.URL)
	cookies := jar.Cookies(srvURL)
	var hasAuthToken bool
	for _, c := range cookies {
		if c.Name == "auth_token" {
			hasAuthToken = true
		}
	}
	if !hasAuthToken {
		t.Fatal("expected auth_token cookie to be set after login")
	}

	// 3. Access protected dashboard route.
	resp, err = client.Get(srv.URL + "/dashboard")
	if err != nil {
		t.Fatalf("GET /dashboard: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard: expected 200, got %d", resp.StatusCode)
	}

	// 4. Access home page — should show user name in navbar.
	resp, err = client.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("home: expected 200, got %d", resp.StatusCode)
	}

	// 5. Logout.
	resp, err = client.PostForm(srv.URL+"/logout", nil)
	if err != nil {
		t.Fatalf("POST /logout: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("logout: expected 303 redirect, got %d", resp.StatusCode)
	}

	// 6. Dashboard should now return 401.
	// Clear the cookie jar to simulate cleared cookie.
	jar, _ = cookiejar.New(nil)
	client.Jar = jar
	resp, err = client.Get(srv.URL + "/dashboard")
	if err != nil {
		t.Fatalf("GET /dashboard after logout: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("dashboard after logout: expected 401, got %d", resp.StatusCode)
	}
}

func TestIntegration_LoginWrongPassword(t *testing.T) {
	auth := newTestAuthService(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Register first.
	resp, err := client.PostForm(srv.URL+"/register", url.Values{
		"email":        {"wrong@example.com"},
		"display_name": {"Wrong PW"},
		"password":     {"password123"},
	})
	if err != nil {
		t.Fatalf("POST /register: %v", err)
	}
	resp.Body.Close()

	// Attempt login with wrong password.
	resp, err = client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"wrong@example.com"},
		"password": {"badpassword"},
	})
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestIntegration_RegisterDuplicateEmail(t *testing.T) {
	auth := newTestAuthService(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	form := url.Values{
		"email":        {"dup@example.com"},
		"display_name": {"Dup User"},
		"password":     {"password123"},
	}

	// Register first time.
	resp, err := client.PostForm(srv.URL+"/register", form)
	if err != nil {
		t.Fatalf("first register: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("first register: expected 303, got %d", resp.StatusCode)
	}

	// Register same email again.
	resp, err = client.PostForm(srv.URL+"/register", form)
	if err != nil {
		t.Fatalf("second register: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate register: expected 422, got %d", resp.StatusCode)
	}
}

func TestIntegration_RegisterWeakPassword(t *testing.T) {
	auth := newTestAuthService(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.PostForm(srv.URL+"/register", url.Values{
		"email":        {"weak@example.com"},
		"display_name": {"Weak PW"},
		"password":     {"short"},
	})
	if err != nil {
		t.Fatalf("POST /register: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("weak password register: expected 422, got %d", resp.StatusCode)
	}
}

func TestIntegration_LoginPageRendering(t *testing.T) {
	auth := newTestAuthService(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bodyBytes)
	if !strings.Contains(body, "Log In") {
		t.Fatal("login page should contain 'Log In'")
	}
}
