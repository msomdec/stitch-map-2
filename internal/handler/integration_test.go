package handler_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
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
	auth, stitches, patterns, sessions, images := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
		"email":            {"integ@example.com"},
		"display_name":     {"Integration User"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
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
	auth, stitches, patterns, sessions, images := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Register first.
	resp, err := client.PostForm(srv.URL+"/register", url.Values{
		"email":            {"wrong@example.com"},
		"display_name":     {"Wrong PW"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
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
	auth, stitches, patterns, sessions, images := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	form := url.Values{
		"email":            {"dup@example.com"},
		"display_name":     {"Dup User"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
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
	auth, stitches, patterns, sessions, images := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.PostForm(srv.URL+"/register", url.Values{
		"email":            {"weak@example.com"},
		"display_name":     {"Weak PW"},
		"password":         {"short"},
		"confirm_password": {"short"},
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
	auth, stitches, patterns, sessions, images := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
	auth, stitches, patterns, sessions, images := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
	auth, stitches, patterns, sessions, images := newTestServices(t)

	// Seed predefined stitches.
	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
		"email":            {"stitch@example.com"},
		"display_name":     {"Stitch User"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
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
	before, _, ok := strings.Cut(rest, "/")
	if !ok {
		t.Fatal("expected /delete path after stitch ID")
	}
	stitchID := before

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
	auth, stitches, patterns, sessions, images := newTestServices(t)

	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
		"email":            {"filter@example.com"},
		"display_name":     {"Filter User"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
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
	auth, stitches, patterns, sessions, images := newTestServices(t)

	// Seed stitches so we can reference them.
	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
		"email":            {"pattern@example.com"},
		"display_name":     {"Pattern User"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
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
		"name":             {"Test Amigurumi"},
		"description":      {"A small amigurumi ball"},
		"pattern_type":     {"round"},
		"hook_size":        {"4.0mm"},
		"yarn_weight":      {"Worsted"},
		"group_label_0":    {"Round 1"},
		"group_repeat_0":   {"1"},
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

func TestIntegration_Pattern_ViewWithTextPreview(t *testing.T) {
	auth, stitches, patterns, sessions, images := newTestServices(t)

	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
		"email":            {"preview@example.com"},
		"display_name":     {"Preview User"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
	})
	client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"preview@example.com"},
		"password": {"password123"},
	})

	// Get sc and inc stitch IDs.
	predefined, _ := stitches.ListPredefined(context.Background())
	scID, incID := "", ""
	for _, s := range predefined {
		switch s.Abbreviation {
		case "sc":
			scID = strconv.FormatInt(s.ID, 10)
		case "inc":
			incID = strconv.FormatInt(s.ID, 10)
		}
	}
	if scID == "" || incID == "" {
		t.Fatal("sc or inc stitch not found")
	}

	// Create a pattern with two groups.
	resp, err := client.PostForm(srv.URL+"/patterns", url.Values{
		"name":             {"Preview Test"},
		"pattern_type":     {"round"},
		"group_label_0":    {"Round 1"},
		"group_repeat_0":   {"1"},
		"entry_stitch_0_0": {scID},
		"entry_count_0_0":  {"6"},
		"entry_repeat_0_0": {"1"},
		"group_label_1":    {"Round 2"},
		"group_repeat_1":   {"1"},
		"entry_stitch_1_0": {incID},
		"entry_count_1_0":  {"1"},
		"entry_repeat_1_0": {"6"},
	})
	if err != nil {
		t.Fatalf("POST /patterns: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("create: expected 303, got %d", resp.StatusCode)
	}

	// Find pattern ID from list.
	resp, err = client.Get(srv.URL + "/patterns")
	if err != nil {
		t.Fatalf("GET /patterns: %v", err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	body := string(bodyBytes)

	patternID := extractPatternID(t, body)

	// View the pattern.
	resp, err = client.Get(srv.URL + "/patterns/" + patternID)
	if err != nil {
		t.Fatalf("GET /patterns/%s: %v", patternID, err)
	}
	bodyBytes, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("view: expected 200, got %d", resp.StatusCode)
	}
	body = string(bodyBytes)

	// Should contain pattern text preview.
	if !strings.Contains(body, "Pattern Text") {
		t.Fatal("pattern view should contain 'Pattern Text' section")
	}
	if !strings.Contains(body, "Round 1: 6 sc") {
		t.Fatal("pattern view should contain rendered text 'Round 1: 6 sc'")
	}
	if !strings.Contains(body, "Round 2:") {
		t.Fatal("pattern view should contain 'Round 2:' in rendered text")
	}
	// Check per-group text rendering in each group box.
	if !strings.Contains(body, "inc") {
		t.Fatal("pattern view should contain stitch abbreviation 'inc'")
	}
}

func TestIntegration_Pattern_EditorPreview(t *testing.T) {
	auth, stitches, patterns, sessions, images := newTestServices(t)

	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
		"email":            {"editorpreview@example.com"},
		"display_name":     {"Editor Preview User"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
	})
	client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"editorpreview@example.com"},
		"password": {"password123"},
	})

	predefined, _ := stitches.ListPredefined(context.Background())
	scID := ""
	for _, s := range predefined {
		if s.Abbreviation == "sc" {
			scID = strconv.FormatInt(s.ID, 10)
			break
		}
	}

	// Create a pattern first.
	resp, err := client.PostForm(srv.URL+"/patterns", url.Values{
		"name":             {"Editor Preview Test"},
		"pattern_type":     {"round"},
		"group_label_0":    {"Round 1"},
		"group_repeat_0":   {"1"},
		"entry_stitch_0_0": {scID},
		"entry_count_0_0":  {"6"},
		"entry_repeat_0_0": {"1"},
	})
	if err != nil {
		t.Fatalf("POST /patterns: %v", err)
	}
	resp.Body.Close()

	// Find pattern ID.
	resp, err = client.Get(srv.URL + "/patterns")
	if err != nil {
		t.Fatalf("GET /patterns: %v", err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	patternID := extractPatternID(t, string(bodyBytes))

	// Open the edit page.
	resp, err = client.Get(srv.URL + "/patterns/" + patternID + "/edit")
	if err != nil {
		t.Fatalf("GET edit: %v", err)
	}
	bodyBytes, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("edit: expected 200, got %d", resp.StatusCode)
	}
	body := string(bodyBytes)

	// Editor should contain the preview panel.
	if !strings.Contains(body, "Preview") {
		t.Fatal("editor should contain 'Preview' panel")
	}
	if !strings.Contains(body, "Round 1: 6 sc") {
		t.Fatal("editor preview should contain rendered text 'Round 1: 6 sc'")
	}
}

func TestIntegration_Pattern_Duplicate(t *testing.T) {
	auth, stitches, patterns, sessions, images := newTestServices(t)

	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
		"email":            {"dup-pattern@example.com"},
		"display_name":     {"Dup User"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
	})
	client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"dup-pattern@example.com"},
		"password": {"password123"},
	})

	predefined, _ := stitches.ListPredefined(context.Background())
	scID := ""
	for _, s := range predefined {
		if s.Abbreviation == "sc" {
			scID = strconv.FormatInt(s.ID, 10)
			break
		}
	}

	// Create a pattern.
	resp, err := client.PostForm(srv.URL+"/patterns", url.Values{
		"name":             {"Original Pattern"},
		"pattern_type":     {"round"},
		"group_label_0":    {"Round 1"},
		"group_repeat_0":   {"1"},
		"entry_stitch_0_0": {scID},
		"entry_count_0_0":  {"6"},
		"entry_repeat_0_0": {"1"},
	})
	if err != nil {
		t.Fatalf("POST /patterns: %v", err)
	}
	resp.Body.Close()

	// Find pattern ID.
	resp, err = client.Get(srv.URL + "/patterns")
	if err != nil {
		t.Fatalf("GET /patterns: %v", err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	patternID := extractPatternID(t, string(bodyBytes))

	// Duplicate.
	resp, err = client.PostForm(srv.URL+"/patterns/"+patternID+"/duplicate", nil)
	if err != nil {
		t.Fatalf("POST duplicate: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("duplicate: expected 303, got %d", resp.StatusCode)
	}

	// Verify both original and copy appear in the list.
	resp, err = client.Get(srv.URL + "/patterns")
	if err != nil {
		t.Fatalf("GET /patterns: %v", err)
	}
	bodyBytes, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	body := string(bodyBytes)

	if !strings.Contains(body, "Original Pattern") {
		t.Fatal("list should contain original pattern")
	}
	if !strings.Contains(body, "Original Pattern (Copy)") {
		t.Fatal("list should contain duplicated pattern with '(Copy)' suffix")
	}

	// Delete the original — the copy should remain.
	resp, err = client.PostForm(srv.URL+"/patterns/"+patternID+"/delete", nil)
	if err != nil {
		t.Fatalf("POST delete: %v", err)
	}
	resp.Body.Close()

	resp, err = client.Get(srv.URL + "/patterns")
	if err != nil {
		t.Fatalf("GET /patterns: %v", err)
	}
	bodyBytes, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	body = string(bodyBytes)

	if !strings.Contains(body, "Original Pattern (Copy)") {
		t.Fatal("copy should still exist after deleting original")
	}
}

// extractPatternID finds the first numeric pattern ID from /patterns/{id} links in HTML.
func extractPatternID(t *testing.T, body string) string {
	t.Helper()
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
			return candidate
		}
		searchBody = rest
	}
	t.Fatal("couldn't extract numeric pattern ID from page")
	return ""
}

func TestIntegration_Pattern_Unauthenticated(t *testing.T) {
	auth, stitches, patterns, sessions, images := newTestServices(t)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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

func TestIntegration_WorkSession_NavigateToCompletion(t *testing.T) {
	auth, stitches, patterns, sessions, images := newTestServices(t)

	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
		"email":            {"session@example.com"},
		"display_name":     {"Session User"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
	})
	client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"session@example.com"},
		"password": {"password123"},
	})

	// Get sc stitch ID.
	predefined, _ := stitches.ListPredefined(context.Background())
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

	// Create a small pattern: Round 1: 3 sc (3 total stitches).
	resp, err := client.PostForm(srv.URL+"/patterns", url.Values{
		"name":             {"Session Test"},
		"pattern_type":     {"round"},
		"group_label_0":    {"Round 1"},
		"group_repeat_0":   {"1"},
		"entry_stitch_0_0": {scID},
		"entry_count_0_0":  {"3"},
		"entry_repeat_0_0": {"1"},
	})
	if err != nil {
		t.Fatalf("POST /patterns: %v", err)
	}
	resp.Body.Close()

	// Find pattern ID.
	resp, err = client.Get(srv.URL + "/patterns")
	if err != nil {
		t.Fatalf("GET /patterns: %v", err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	patternID := extractPatternID(t, string(bodyBytes))

	// Start a work session.
	resp, err = client.PostForm(srv.URL+"/patterns/"+patternID+"/start-session", nil)
	if err != nil {
		t.Fatalf("POST start-session: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("start session: expected 303, got %d", resp.StatusCode)
	}
	sessionURL := resp.Header.Get("Location")
	if sessionURL == "" {
		t.Fatal("expected redirect location for session")
	}

	// View the session.
	resp, err = client.Get(srv.URL + sessionURL)
	if err != nil {
		t.Fatalf("GET session: %v", err)
	}
	bodyBytes, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("view session: expected 200, got %d", resp.StatusCode)
	}
	body := string(bodyBytes)
	if !strings.Contains(body, "sc") {
		t.Fatal("session page should contain current stitch abbreviation 'sc'")
	}
	if !strings.Contains(body, "Round 1") {
		t.Fatal("session page should contain group label 'Round 1'")
	}

	// Navigate forward 3 times to complete the pattern.
	for i := range 3 {
		resp, err = client.PostForm(srv.URL+sessionURL+"/next", nil)
		if err != nil {
			t.Fatalf("POST next (stitch %d): %v", i+1, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("next stitch %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}

	// Session should now be completed — verify via GET (full page render).
	resp, err = client.Get(srv.URL + sessionURL)
	if err != nil {
		t.Fatalf("GET session after completion: %v", err)
	}
	bodyBytes, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	body = string(bodyBytes)
	if !strings.Contains(body, "Pattern Complete") {
		t.Fatal("session page should show 'Pattern Complete' after navigating through all stitches")
	}
}

func TestIntegration_WorkSession_MultiGroupNavigateToCompletion(t *testing.T) {
	auth, stitches, patterns, sessions, images := newTestServices(t)

	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
		"email":            {"multigroup@example.com"},
		"display_name":     {"MultiGroup User"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
	})
	client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"multigroup@example.com"},
		"password": {"password123"},
	})

	// Get sc and dc stitch IDs.
	predefined, _ := stitches.ListPredefined(context.Background())
	scID, dcID := "", ""
	for _, s := range predefined {
		switch s.Abbreviation {
		case "sc":
			scID = strconv.FormatInt(s.ID, 10)
		case "dc":
			dcID = strconv.FormatInt(s.ID, 10)
		}
	}
	if scID == "" || dcID == "" {
		t.Fatal("sc or dc stitch not found")
	}

	// Create a 2-group pattern:
	//   Group 0 (Round 1): 3x sc = 3 stitches
	//   Group 1 (Round 2): 3x dc = 3 stitches
	//   Total: 6 stitches
	resp, err := client.PostForm(srv.URL+"/patterns", url.Values{
		"name":             {"Multi-Group Test"},
		"pattern_type":     {"round"},
		"group_label_0":    {"Round 1"},
		"group_repeat_0":   {"1"},
		"entry_stitch_0_0": {scID},
		"entry_count_0_0":  {"3"},
		"entry_repeat_0_0": {"1"},
		"group_label_1":    {"Round 2"},
		"group_repeat_1":   {"1"},
		"entry_stitch_1_0": {dcID},
		"entry_count_1_0":  {"3"},
		"entry_repeat_1_0": {"1"},
	})
	if err != nil {
		t.Fatalf("POST /patterns: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("create pattern: expected 303, got %d", resp.StatusCode)
	}

	// Find pattern ID.
	resp, err = client.Get(srv.URL + "/patterns")
	if err != nil {
		t.Fatalf("GET /patterns: %v", err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	patternID := extractPatternID(t, string(bodyBytes))

	// Start a work session.
	resp, err = client.PostForm(srv.URL+"/patterns/"+patternID+"/start-session", nil)
	if err != nil {
		t.Fatalf("POST start-session: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("start session: expected 303, got %d", resp.StatusCode)
	}
	sessionURL := resp.Header.Get("Location")
	if sessionURL == "" {
		t.Fatal("expected redirect location for session")
	}

	// Navigate forward 3 times (complete group 0).
	for i := range 3 {
		resp, err = client.PostForm(srv.URL+sessionURL+"/next", nil)
		if err != nil {
			t.Fatalf("POST next (group 0, stitch %d): %v", i+1, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("next group 0 stitch %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}

	// After completing group 0, the session should still be active (NOT completed).
	resp, err = client.Get(srv.URL + sessionURL)
	if err != nil {
		t.Fatalf("GET session after group 0: %v", err)
	}
	bodyBytes, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	body := string(bodyBytes)
	if strings.Contains(body, "Pattern Complete") {
		t.Fatal("session should NOT be completed after finishing only group 0 — group 1 remains")
	}
	if !strings.Contains(body, "Round 2") {
		t.Fatal("session should show 'Round 2' after completing group 0")
	}

	// Navigate forward 3 more times (complete group 1).
	for i := range 3 {
		resp, err = client.PostForm(srv.URL+sessionURL+"/next", nil)
		if err != nil {
			t.Fatalf("POST next (group 1, stitch %d): %v", i+1, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("next group 1 stitch %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}

	// Now the session should be completed.
	resp, err = client.Get(srv.URL + sessionURL)
	if err != nil {
		t.Fatalf("GET session after completion: %v", err)
	}
	bodyBytes, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	body = string(bodyBytes)
	if !strings.Contains(body, "Pattern Complete") {
		t.Fatal("session should show 'Pattern Complete' after navigating through all stitches in both groups")
	}
}

func TestIntegration_WorkSession_NavigateBackward(t *testing.T) {
	auth, stitches, patterns, sessions, images := newTestServices(t)

	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
		"email":            {"backward@example.com"},
		"display_name":     {"Backward User"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
	})
	client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"backward@example.com"},
		"password": {"password123"},
	})

	predefined, _ := stitches.ListPredefined(context.Background())
	scID := ""
	for _, s := range predefined {
		if s.Abbreviation == "sc" {
			scID = strconv.FormatInt(s.ID, 10)
			break
		}
	}

	// Create pattern: Round 1: 4 sc.
	resp, err := client.PostForm(srv.URL+"/patterns", url.Values{
		"name":             {"Backward Test"},
		"pattern_type":     {"round"},
		"group_label_0":    {"Round 1"},
		"group_repeat_0":   {"1"},
		"entry_stitch_0_0": {scID},
		"entry_count_0_0":  {"4"},
		"entry_repeat_0_0": {"1"},
	})
	if err != nil {
		t.Fatalf("POST /patterns: %v", err)
	}
	resp.Body.Close()

	resp, err = client.Get(srv.URL + "/patterns")
	if err != nil {
		t.Fatalf("GET /patterns: %v", err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	patternID := extractPatternID(t, string(bodyBytes))

	// Start session.
	resp, err = client.PostForm(srv.URL+"/patterns/"+patternID+"/start-session", nil)
	if err != nil {
		t.Fatalf("POST start-session: %v", err)
	}
	resp.Body.Close()
	sessionURL := resp.Header.Get("Location")

	// Navigate forward 2 times.
	for range 2 {
		resp, err = client.PostForm(srv.URL+sessionURL+"/next", nil)
		if err != nil {
			t.Fatalf("POST next: %v", err)
		}
		resp.Body.Close()
	}

	// Navigate backward 1 time.
	resp, err = client.PostForm(srv.URL+sessionURL+"/prev", nil)
	if err != nil {
		t.Fatalf("POST prev: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("prev: expected 200, got %d", resp.StatusCode)
	}

	// View session — should show progress at 1/4 (stitch count position 1).
	resp, err = client.Get(srv.URL + sessionURL)
	if err != nil {
		t.Fatalf("GET session: %v", err)
	}
	bodyBytes, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	body := string(bodyBytes)

	// Should still be active (not completed).
	if strings.Contains(body, "Pattern Complete") {
		t.Fatal("session should not be completed after navigating backward")
	}
	if !strings.Contains(body, "sc") {
		t.Fatal("session should show current stitch 'sc'")
	}
}

// TestFullHappyPath is the complete regression test covering the entire user journey:
// register -> create custom stitch -> create pattern -> preview -> start session ->
// navigate to completion -> verify session marked complete.
func TestFullHappyPath(t *testing.T) {
	auth, stitches, patterns, sessions, images := newTestServices(t)

	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 1. Register a new user.
	resp, err := client.PostForm(srv.URL+"/register", url.Values{
		"email":            {"happy@example.com"},
		"display_name":     {"Happy Crocheter"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("register: expected 303, got %d", resp.StatusCode)
	}

	// 2. Login.
	resp, err = client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"happy@example.com"},
		"password": {"password123"},
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: expected 303, got %d", resp.StatusCode)
	}

	// Verify dashboard is accessible.
	resp, err = client.Get(srv.URL + "/dashboard")
	if err != nil {
		t.Fatalf("GET /dashboard: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard: expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "Happy Crocheter") {
		t.Fatal("dashboard should show user display name")
	}

	// 3. Create a custom stitch.
	resp, err = client.PostForm(srv.URL+"/stitches", url.Values{
		"abbreviation": {"msc"},
		"name":         {"Magic Single Crochet"},
		"category":     {"custom"},
		"description":  {"A magical stitch"},
	})
	if err != nil {
		t.Fatalf("create custom stitch: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("create custom stitch: expected 303, got %d", resp.StatusCode)
	}

	// Verify stitch appears in library.
	resp, err = client.Get(srv.URL + "/stitches")
	if err != nil {
		t.Fatalf("GET /stitches: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "msc") {
		t.Fatal("stitch library should show custom stitch 'msc'")
	}

	// 4. Create a pattern using a predefined stitch (sc).
	predefined, err := stitches.ListPredefined(context.Background())
	if err != nil || len(predefined) == 0 {
		t.Fatalf("ListPredefined: %v (count: %d)", err, len(predefined))
	}
	scID := ""
	for _, s := range predefined {
		if s.Abbreviation == "sc" {
			scID = strconv.FormatInt(s.ID, 10)
			break
		}
	}
	if scID == "" {
		t.Fatal("predefined 'sc' stitch not found")
	}

	resp, err = client.PostForm(srv.URL+"/patterns", url.Values{
		"name":             {"Happy Path Pattern"},
		"pattern_type":     {"round"},
		"hook_size":        {"5.0mm"},
		"yarn_weight":      {"Worsted"},
		"description":      {"A full happy path test pattern"},
		"group_label_0":    {"Round 1"},
		"group_repeat_0":   {"1"},
		"entry_stitch_0_0": {scID},
		"entry_count_0_0":  {"3"},
		"entry_repeat_0_0": {"1"},
	})
	if err != nil {
		t.Fatalf("create pattern: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("create pattern: expected 303, got %d", resp.StatusCode)
	}

	// 5. View pattern list and find the pattern.
	resp, err = client.Get(srv.URL + "/patterns")
	if err != nil {
		t.Fatalf("GET /patterns: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "Happy Path Pattern") {
		t.Fatal("pattern list should contain the new pattern")
	}
	patternID := extractPatternID(t, string(body))

	// 6. View the pattern detail (preview).
	resp, err = client.Get(srv.URL + "/patterns/" + patternID)
	if err != nil {
		t.Fatalf("GET pattern detail: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pattern detail: expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "Round 1") {
		t.Fatal("pattern detail should show instruction group 'Round 1'")
	}

	// 7. Start a work session.
	resp, err = client.PostForm(srv.URL+"/patterns/"+patternID+"/start-session", nil)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("start session: expected 303, got %d", resp.StatusCode)
	}
	sessionURL := resp.Header.Get("Location")
	if sessionURL == "" {
		t.Fatal("expected session redirect location")
	}

	// Verify session view shows current stitch.
	resp, err = client.Get(srv.URL + sessionURL)
	if err != nil {
		t.Fatalf("GET session: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("session view: expected 200, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "sc") {
		t.Fatal("session page should show current stitch 'sc'")
	}

	// 8. Navigate forward through all 3 stitches to complete.
	for i := range 3 {
		resp, err = client.PostForm(srv.URL+sessionURL+"/next", nil)
		if err != nil {
			t.Fatalf("next stitch %d: %v", i+1, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("next stitch %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}

	// 9. Verify session is marked complete.
	resp, err = client.Get(srv.URL + sessionURL)
	if err != nil {
		t.Fatalf("GET session after completion: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "Pattern Complete") {
		t.Fatal("session page should show 'Pattern Complete' after navigating through all stitches")
	}

	// 10. Dashboard should still be accessible.
	resp, err = client.Get(srv.URL + "/dashboard")
	if err != nil {
		t.Fatalf("GET /dashboard after completion: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard after completion: expected 200, got %d", resp.StatusCode)
	}
}

func TestIntegration_ImageUploadAndServe(t *testing.T) {
	auth, stitches, patterns, sessions, images := newTestServices(t)

	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
		"email":            {"img@example.com"},
		"display_name":     {"Image User"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
	})
	client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"img@example.com"},
		"password": {"password123"},
	})

	// Get sc stitch ID.
	predefined, _ := stitches.ListPredefined(context.Background())
	scID := ""
	for _, s := range predefined {
		if s.Abbreviation == "sc" {
			scID = strconv.FormatInt(s.ID, 10)
			break
		}
	}

	// Create a pattern.
	resp, err := client.PostForm(srv.URL+"/patterns", url.Values{
		"name":             {"Image Test Pattern"},
		"pattern_type":     {"round"},
		"group_label_0":    {"Round 1"},
		"group_repeat_0":   {"1"},
		"entry_stitch_0_0": {scID},
		"entry_count_0_0":  {"6"},
		"entry_repeat_0_0": {"1"},
	})
	if err != nil {
		t.Fatalf("create pattern: %v", err)
	}
	resp.Body.Close()

	// Find the pattern ID.
	resp, _ = client.Get(srv.URL + "/patterns")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	patternID := extractPatternID(t, string(body))

	// 1. Upload a valid PNG image.
	pngData := createTestPNG()
	resp, err = uploadImage(client, srv.URL, patternID, "0", "test.png", "image/png", pngData)
	if err != nil {
		t.Fatalf("upload PNG: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload PNG: expected 200, got %d", resp.StatusCode)
	}

	// 2. Upload a valid JPEG image.
	jpegData := createTestJPEG()
	resp, err = uploadImage(client, srv.URL, patternID, "0", "test.jpg", "image/jpeg", jpegData)
	if err != nil {
		t.Fatalf("upload JPEG: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload JPEG: expected 200, got %d", resp.StatusCode)
	}

	// 3. Edit page should show the images (2 / 5 images).
	resp, _ = client.Get(srv.URL + "/patterns/" + patternID + "/edit")
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "2 / 5 images") {
		t.Fatalf("edit page should show '2 / 5 images', body contains: %s", string(body))
	}

	// 4. Serve an image (find image URL from the edit page).
	imageURL := extractImageURL(string(body))
	if imageURL == "" {
		t.Fatal("expected to find an image URL in the edit page")
	}
	resp, err = client.Get(srv.URL + imageURL)
	if err != nil {
		t.Fatalf("GET image: %v", err)
	}
	imgBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("serve image: expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "image/png" && ct != "image/jpeg" {
		t.Fatalf("expected image content type, got %s", ct)
	}
	if len(imgBody) == 0 {
		t.Fatal("image body should not be empty")
	}

	// 5. Upload wrong content type should fail.
	resp, err = uploadImage(client, srv.URL, patternID, "0", "test.gif", "image/gif", []byte("GIF89a"))
	if err != nil {
		t.Fatalf("upload GIF: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("upload GIF: expected 400, got %d", resp.StatusCode)
	}

	// 6. View page should show images in gallery.
	resp, _ = client.Get(srv.URL + "/patterns/" + patternID)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "/images/") {
		t.Fatal("view page should contain image references")
	}
}

func TestIntegration_ImageUploadLimits(t *testing.T) {
	auth, stitches, patterns, sessions, images := newTestServices(t)

	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
		"email":            {"limit@example.com"},
		"display_name":     {"Limit User"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
	})
	client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"limit@example.com"},
		"password": {"password123"},
	})

	predefined, _ := stitches.ListPredefined(context.Background())
	scID := ""
	for _, s := range predefined {
		if s.Abbreviation == "sc" {
			scID = strconv.FormatInt(s.ID, 10)
			break
		}
	}

	// Create a pattern.
	resp, _ := client.PostForm(srv.URL+"/patterns", url.Values{
		"name":             {"Limit Test Pattern"},
		"pattern_type":     {"round"},
		"group_label_0":    {"Round 1"},
		"group_repeat_0":   {"1"},
		"entry_stitch_0_0": {scID},
		"entry_count_0_0":  {"6"},
		"entry_repeat_0_0": {"1"},
	})
	resp.Body.Close()

	respList, _ := client.Get(srv.URL + "/patterns")
	body, _ := io.ReadAll(respList.Body)
	respList.Body.Close()
	patternID := extractPatternID(t, string(body))

	// Upload 5 images to reach the limit.
	for i := range 5 {
		pngData := createTestPNG()
		resp, err := uploadImage(client, srv.URL, patternID, "0", "img"+strconv.Itoa(i)+".png", "image/png", pngData)
		if err != nil {
			t.Fatalf("upload %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("upload %d: expected 200, got %d", i, resp.StatusCode)
		}
	}

	// 6th image should be rejected.
	pngData := createTestPNG()
	resp, err := uploadImage(client, srv.URL, patternID, "0", "extra.png", "image/png", pngData)
	if err != nil {
		t.Fatalf("upload 6th: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("6th upload: expected 400, got %d", resp.StatusCode)
	}
}

func TestIntegration_ImageDeleteAndCascade(t *testing.T) {
	auth, stitches, patterns, sessions, images := newTestServices(t)

	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)

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
		"email":            {"cascade@example.com"},
		"display_name":     {"Cascade User"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
	})
	client.PostForm(srv.URL+"/login", url.Values{
		"email":    {"cascade@example.com"},
		"password": {"password123"},
	})

	predefined, _ := stitches.ListPredefined(context.Background())
	scID := ""
	for _, s := range predefined {
		if s.Abbreviation == "sc" {
			scID = strconv.FormatInt(s.ID, 10)
			break
		}
	}

	// Create a pattern.
	resp, _ := client.PostForm(srv.URL+"/patterns", url.Values{
		"name":             {"Cascade Test Pattern"},
		"pattern_type":     {"round"},
		"group_label_0":    {"Round 1"},
		"group_repeat_0":   {"1"},
		"entry_stitch_0_0": {scID},
		"entry_count_0_0":  {"6"},
		"entry_repeat_0_0": {"1"},
	})
	resp.Body.Close()

	respList, _ := client.Get(srv.URL + "/patterns")
	body, _ := io.ReadAll(respList.Body)
	respList.Body.Close()
	patternID := extractPatternID(t, string(body))

	// Upload an image.
	pngData := createTestPNG()
	resp, _ = uploadImage(client, srv.URL, patternID, "0", "cascade.png", "image/png", pngData)
	resp.Body.Close()

	// Get the image URL from the edit page.
	resp, _ = client.Get(srv.URL + "/patterns/" + patternID + "/edit")
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	imageURL := extractImageURL(string(body))
	if imageURL == "" {
		t.Fatal("expected image URL on edit page after upload")
	}

	// Verify image is accessible.
	resp, _ = client.Get(srv.URL + imageURL)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("image should be accessible, got %d", resp.StatusCode)
	}

	// Delete the pattern (cascade should remove images).
	resp, _ = client.PostForm(srv.URL+"/patterns/"+patternID+"/delete", nil)
	resp.Body.Close()

	// Image should no longer be accessible (404 or similar).
	resp, _ = client.Get(srv.URL + imageURL)
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatal("image should not be accessible after pattern deletion")
	}
}

// uploadImage creates a multipart form request with the given image data.
func uploadImage(client *http.Client, baseURL, patternID, groupIndex, filename, contentType string, data []byte) (*http.Response, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		return nil, err
	}
	part.Write(data)
	writer.Close()

	req, err := http.NewRequest("POST", baseURL+"/patterns/"+patternID+"/parts/"+groupIndex+"/images", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return client.Do(req)
}

// createTestPNG returns a minimal valid PNG file.
func createTestPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// createTestJPEG returns a minimal valid JPEG file.
func createTestJPEG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{0, 255, 0, 255})
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

// extractImageURL finds the first /images/{id} URL in the page body.
func extractImageURL(body string) string {
	idx := strings.Index(body, "/images/")
	if idx == -1 {
		return ""
	}
	rest := body[idx:]
	endIdx := strings.IndexAny(rest, "\"' >")
	if endIdx == -1 {
		return rest
	}
	return rest[:endIdx]
}
