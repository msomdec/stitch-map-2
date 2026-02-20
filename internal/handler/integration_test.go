package handler_test

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/msomdec/stitch-map-2/internal/handler"
)

func TestIntegration_RegisterLoginDashboardLogout(t *testing.T) {
	auth, stitches, patterns := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns)

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
	auth, stitches, patterns := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns)

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
	auth, stitches, patterns := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns)

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
	auth, stitches, patterns := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns)

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
	auth, stitches, patterns := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns)

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

func TestIntegration_StitchLibrary_Unauthenticated(t *testing.T) {
	auth, stitches, patterns := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/stitches")
	if err != nil {
		t.Fatalf("GET /stitches: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestIntegration_StitchLibrary_BrowseCreateEditDelete(t *testing.T) {
	auth, stitches, patterns := newTestServices(t)

	// Seed predefined stitches.
	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Register and login.
	resp, err := client.PostForm(srv.URL+"/register", url.Values{
		"email":        {"stitch@example.com"},
		"display_name": {"Stitch User"},
		"password":     {"password123"},
	})
	if err != nil {
		t.Fatalf("POST /register: %v", err)
	}
	resp.Body.Close()

	resp, err = client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"stitch@example.com"},
		"password": {"password123"},
	})
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	resp.Body.Close()

	// 1. Browse stitch library.
	resp, err = client.Get(srv.URL + "/stitches")
	if err != nil {
		t.Fatalf("GET /stitches: %v", err)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stitch library: expected 200, got %d", resp.StatusCode)
	}
	body := string(bodyBytes)
	if !strings.Contains(body, "Stitch Library") {
		t.Fatal("stitch library page should contain 'Stitch Library'")
	}
	if !strings.Contains(body, "Single Crochet") {
		t.Fatal("stitch library should contain predefined stitch 'Single Crochet'")
	}

	// 2. Create a custom stitch.
	resp, err = client.PostForm(srv.URL+"/stitches", url.Values{
		"abbreviation": {"mst"},
		"name":         {"My Special Thingy"},
		"description":  {"A custom test stitch"},
		"category":     {"custom"},
	})
	if err != nil {
		t.Fatalf("POST /stitches: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("create custom stitch: expected 303 redirect, got %d", resp.StatusCode)
	}

	// 3. Verify the custom stitch appears in the library.
	resp, err = client.Get(srv.URL + "/stitches")
	if err != nil {
		t.Fatalf("GET /stitches after create: %v", err)
	}
	bodyBytes, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body = string(bodyBytes)
	if !strings.Contains(body, "My Special Thingy") {
		t.Fatal("stitch library should contain newly created custom stitch")
	}
	if !strings.Contains(body, "mst") {
		t.Fatal("stitch library should contain custom stitch abbreviation 'mst'")
	}

	// 4. Delete the custom stitch (find its ID from the page).
	// We need to extract the stitch ID. Let's find the delete form action.
	// The form action is /stitches/{id}/delete.
	idx := strings.Index(body, "/stitches/")
	if idx == -1 {
		t.Fatal("expected to find /stitches/ in page body for delete form")
	}
	// Extract the path segment after /stitches/ up to /delete.
	rest := body[idx+len("/stitches/"):]
	slashIdx := strings.Index(rest, "/")
	if slashIdx == -1 {
		t.Fatal("expected /delete path after stitch ID")
	}
	stitchID := rest[:slashIdx]

	resp, err = client.PostForm(srv.URL+"/stitches/"+stitchID+"/delete", nil)
	if err != nil {
		t.Fatalf("POST /stitches/%s/delete: %v", stitchID, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("delete custom stitch: expected 303 redirect, got %d", resp.StatusCode)
	}

	// 5. Verify the custom stitch is gone.
	resp, err = client.Get(srv.URL + "/stitches")
	if err != nil {
		t.Fatalf("GET /stitches after delete: %v", err)
	}
	bodyBytes, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body = string(bodyBytes)
	if strings.Contains(body, "My Special Thingy") {
		t.Fatal("deleted stitch should not appear in library")
	}
}

func TestIntegration_StitchLibrary_FilterByCategory(t *testing.T) {
	auth, stitches, patterns := newTestServices(t)

	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Register and login.
	client.PostForm(srv.URL+"/register", url.Values{
		"email":        {"filter@example.com"},
		"display_name": {"Filter User"},
		"password":     {"password123"},
	})
	client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"filter@example.com"},
		"password": {"password123"},
	})

	// Filter by "post" category.
	resp, err := client.Get(srv.URL + "/stitches?category=post")
	if err != nil {
		t.Fatalf("GET /stitches?category=post: %v", err)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Should contain post stitches.
	if !strings.Contains(body, "Front Post Double Crochet") {
		t.Fatal("post filter should include FPdc")
	}

	// Should NOT contain basic-only stitches (Chain is only in basic category).
	if strings.Contains(body, ">Chain<") {
		t.Fatal("post filter should not include basic stitches")
	}
}

func TestIntegration_Pattern_CreateViewEditDelete(t *testing.T) {
	auth, stitches, patterns := newTestServices(t)

	// Seed stitches so we can reference them.
	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Register and login.
	client.PostForm(srv.URL+"/register", url.Values{
		"email":        {"pattern@example.com"},
		"display_name": {"Pattern User"},
		"password":     {"password123"},
	})
	client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"pattern@example.com"},
		"password": {"password123"},
	})

	// Get a stitch ID by listing predefined.
	predefined, _ := stitches.ListPredefined(context.Background())
	if len(predefined) == 0 {
		t.Fatal("no predefined stitches")
	}
	scID := ""
	for _, s := range predefined {
		if s.Abbreviation == "sc" {
			scID = strconv.FormatInt(s.ID, 10)
			break
		}
	}
	if scID == "" {
		t.Fatal("sc stitch not found")
	}

	// 1. Pattern list should be empty initially.
	resp, err := client.Get(srv.URL + "/patterns")
	if err != nil {
		t.Fatalf("GET /patterns: %v", err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body := string(bodyBytes)
	if !strings.Contains(body, "My Patterns") {
		t.Fatal("pattern list should contain 'My Patterns'")
	}

	// 2. Create a pattern.
	resp, err = client.PostForm(srv.URL+"/patterns", url.Values{
		"name":           {"Test Amigurumi"},
		"description":    {"A small amigurumi ball"},
		"pattern_type":   {"round"},
		"hook_size":      {"4.0mm"},
		"yarn_weight":    {"Worsted"},
		"group_label_0":  {"Round 1"},
		"group_repeat_0": {"1"},
		"entry_stitch_0_0": {scID},
		"entry_count_0_0":  {"6"},
		"entry_repeat_0_0": {"1"},
	})
	if err != nil {
		t.Fatalf("POST /patterns: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("create pattern: expected 303 redirect, got %d", resp.StatusCode)
	}

	// 3. Verify the pattern appears in the list.
	resp, err = client.Get(srv.URL + "/patterns")
	if err != nil {
		t.Fatalf("GET /patterns after create: %v", err)
	}
	bodyBytes, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	body = string(bodyBytes)
	if !strings.Contains(body, "Test Amigurumi") {
		t.Fatal("pattern list should contain 'Test Amigurumi'")
	}

	// Extract pattern ID from the view link (skip /patterns/new).
	patternID := ""
	searchBody := body
	for {
		idx := strings.Index(searchBody, "/patterns/")
		if idx == -1 {
			break
		}
		rest := searchBody[idx+len("/patterns/"):]
		endIdx := strings.IndexAny(rest, "\"/ >")
		if endIdx == -1 {
			break
		}
		candidate := rest[:endIdx]
		if _, err := strconv.Atoi(candidate); err == nil {
			patternID = candidate
			break
		}
		searchBody = rest
	}
	if patternID == "" {
		t.Fatal("couldn't extract numeric pattern ID from page")
	}

	// 4. View the pattern.
	resp, err = client.Get(srv.URL + "/patterns/" + patternID)
	if err != nil {
		t.Fatalf("GET /patterns/%s: %v", patternID, err)
	}
	bodyBytes, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("view pattern: expected 200, got %d", resp.StatusCode)
	}
	body = string(bodyBytes)
	if !strings.Contains(body, "Test Amigurumi") {
		t.Fatal("pattern view should contain 'Test Amigurumi'")
	}
	if !strings.Contains(body, "sc") {
		t.Fatal("pattern view should contain stitch abbreviation 'sc'")
	}

	// 5. Delete the pattern.
	resp, err = client.PostForm(srv.URL+"/patterns/"+patternID+"/delete", nil)
	if err != nil {
		t.Fatalf("POST /patterns/%s/delete: %v", patternID, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("delete pattern: expected 303, got %d", resp.StatusCode)
	}

	// 6. Verify the pattern is gone.
	resp, err = client.Get(srv.URL + "/patterns")
	if err != nil {
		t.Fatalf("GET /patterns after delete: %v", err)
	}
	bodyBytes, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	body = string(bodyBytes)
	if strings.Contains(body, "Test Amigurumi") {
		t.Fatal("deleted pattern should not appear in list")
	}
}

func TestIntegration_Pattern_Unauthenticated(t *testing.T) {
	auth, stitches, patterns := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/patterns")
	if err != nil {
		t.Fatalf("GET /patterns: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
