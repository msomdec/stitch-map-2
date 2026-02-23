package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
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
	"testing"

	"github.com/msomdec/stitch-map-2/internal/handler"
)

// jsonBody encodes the given value to a JSON bytes.Buffer for use as a request body.
func jsonBody(v any) *bytes.Buffer {
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(v)
	return buf
}

// parseJSON decodes a JSON response body into the given destination.
func parseJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		t.Fatalf("decode JSON: %v\nbody: %s", err, string(body))
	}
}

// setupTestServer creates a test server with all routes registered and returns
// the server, a cookie-jar-enabled client, and an auth service reference.
func setupTestServer(t *testing.T) (*httptest.Server, *http.Client) {
	t.Helper()
	auth, stitches, patterns, sessions, images := newTestServices(t)
	if err := stitches.SeedPredefined(context.Background()); err != nil {
		t.Fatalf("SeedPredefined: %v", err)
	}
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, auth, stitches, patterns, sessions, images)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	return srv, client
}

// registerAndLogin registers a user and logs them in, returning the client
// with auth cookies set.
func registerAndLogin(t *testing.T, srv *httptest.Server, client *http.Client, email, displayName, password string) {
	t.Helper()
	// Register.
	resp, err := client.Post(srv.URL+"/api/auth/register", "application/json",
		jsonBody(map[string]string{
			"email":           email,
			"displayName":     displayName,
			"password":        password,
			"confirmPassword": password,
		}))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", resp.StatusCode)
	}

	// Login.
	resp, err = client.Post(srv.URL+"/api/auth/login", "application/json",
		jsonBody(map[string]string{
			"email":    email,
			"password": password,
		}))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", resp.StatusCode)
	}

	// Verify auth_token cookie was set.
	srvURL, _ := url.Parse(srv.URL)
	cookies := client.Jar.Cookies(srvURL)
	var hasAuthToken bool
	for _, c := range cookies {
		if c.Name == "auth_token" {
			hasAuthToken = true
		}
	}
	if !hasAuthToken {
		t.Fatal("expected auth_token cookie to be set after login")
	}
}

func TestIntegration_RegisterLoginDashboardLogout(t *testing.T) {
	srv, client := setupTestServer(t)

	registerAndLogin(t, srv, client, "integ@example.com", "Integration User", "password123")

	// Access dashboard.
	resp, err := client.Get(srv.URL + "/api/dashboard")
	if err != nil {
		t.Fatalf("GET /api/dashboard: %v", err)
	}
	var dashResp map[string]any
	parseJSON(t, resp, &dashResp)
	if _, ok := dashResp["activeSessions"]; !ok {
		t.Fatal("dashboard response should contain 'activeSessions'")
	}

	// /api/auth/me should return user.
	resp, err = client.Get(srv.URL + "/api/auth/me")
	if err != nil {
		t.Fatalf("GET /api/auth/me: %v", err)
	}
	var meResp map[string]any
	parseJSON(t, resp, &meResp)
	user := meResp["user"].(map[string]any)
	if user["displayName"] != "Integration User" {
		t.Fatalf("expected 'Integration User', got %v", user["displayName"])
	}

	// Logout.
	resp, err = client.Post(srv.URL+"/api/auth/logout", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/auth/logout: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout: expected 204, got %d", resp.StatusCode)
	}

	// Dashboard should return 401 after logout.
	// Clear the cookie jar to simulate cookie being cleared.
	jar, _ := cookiejar.New(nil)
	client.Jar = jar
	resp, err = client.Get(srv.URL + "/api/dashboard")
	if err != nil {
		t.Fatalf("GET /api/dashboard after logout: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("dashboard after logout: expected 401, got %d", resp.StatusCode)
	}
}

func TestIntegration_LoginWrongPassword(t *testing.T) {
	srv, client := setupTestServer(t)

	// Register first.
	resp, err := client.Post(srv.URL+"/api/auth/register", "application/json",
		jsonBody(map[string]string{
			"email":           "wrong@example.com",
			"displayName":     "Wrong PW",
			"password":        "password123",
			"confirmPassword": "password123",
		}))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	resp.Body.Close()

	// Attempt login with wrong password.
	resp, err = client.Post(srv.URL+"/api/auth/login", "application/json",
		jsonBody(map[string]string{
			"email":    "wrong@example.com",
			"password": "badpassword",
		}))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestIntegration_RegisterDuplicateEmail(t *testing.T) {
	srv, client := setupTestServer(t)

	regBody := map[string]string{
		"email":           "dup@example.com",
		"displayName":     "Dup User",
		"password":        "password123",
		"confirmPassword": "password123",
	}

	// First registration.
	resp, err := client.Post(srv.URL+"/api/auth/register", "application/json", jsonBody(regBody))
	if err != nil {
		t.Fatalf("first register: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first register: expected 201, got %d", resp.StatusCode)
	}

	// Duplicate registration.
	resp, err = client.Post(srv.URL+"/api/auth/register", "application/json", jsonBody(regBody))
	if err != nil {
		t.Fatalf("second register: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate register: expected 409, got %d", resp.StatusCode)
	}
}

func TestIntegration_RegisterWeakPassword(t *testing.T) {
	srv, client := setupTestServer(t)

	resp, err := client.Post(srv.URL+"/api/auth/register", "application/json",
		jsonBody(map[string]string{
			"email":           "weak@example.com",
			"displayName":     "Weak PW",
			"password":        "short",
			"confirmPassword": "short",
		}))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("weak password register: expected 422, got %d", resp.StatusCode)
	}
}

func TestIntegration_StitchLibrary_Unauthenticated(t *testing.T) {
	srv, client := setupTestServer(t)

	resp, err := client.Get(srv.URL + "/api/stitches")
	if err != nil {
		t.Fatalf("GET /api/stitches: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestIntegration_StitchLibrary_BrowseCreateDelete(t *testing.T) {
	srv, client := setupTestServer(t)
	registerAndLogin(t, srv, client, "stitch@example.com", "Stitch User", "password123")

	// 1. Browse stitch library.
	resp, err := client.Get(srv.URL + "/api/stitches")
	if err != nil {
		t.Fatalf("GET /api/stitches: %v", err)
	}
	var libResp map[string]any
	parseJSON(t, resp, &libResp)

	predefined := libResp["predefined"].([]any)
	if len(predefined) == 0 {
		t.Fatal("stitch library should have predefined stitches")
	}

	// 2. Create a custom stitch.
	resp, err = client.Post(srv.URL+"/api/stitches", "application/json",
		jsonBody(map[string]string{
			"abbreviation": "mst",
			"name":         "My Special Thingy",
			"description":  "A custom test stitch",
			"category":     "custom",
		}))
	if err != nil {
		t.Fatalf("POST /api/stitches: %v", err)
	}
	var createResp map[string]any
	parseJSON(t, resp, &createResp)
	stitchObj := createResp["stitch"].(map[string]any)
	stitchID := strconv.FormatInt(int64(stitchObj["id"].(float64)), 10)

	// 3. Verify the custom stitch appears.
	resp, err = client.Get(srv.URL + "/api/stitches")
	if err != nil {
		t.Fatalf("GET /api/stitches: %v", err)
	}
	parseJSON(t, resp, &libResp)
	custom := libResp["custom"].([]any)
	found := false
	for _, s := range custom {
		st := s.(map[string]any)
		if st["abbreviation"] == "mst" {
			found = true
		}
	}
	if !found {
		t.Fatal("stitch library should contain newly created custom stitch 'mst'")
	}

	// 4. Delete the custom stitch.
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/stitches/"+stitchID, nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/stitches/%s: %v", stitchID, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete stitch: expected 204, got %d", resp.StatusCode)
	}

	// 5. Verify it's gone.
	resp, err = client.Get(srv.URL + "/api/stitches")
	if err != nil {
		t.Fatalf("GET /api/stitches after delete: %v", err)
	}
	parseJSON(t, resp, &libResp)
	custom = libResp["custom"].([]any)
	for _, s := range custom {
		st := s.(map[string]any)
		if st["abbreviation"] == "mst" {
			t.Fatal("deleted stitch should not appear in library")
		}
	}
}

func TestIntegration_StitchLibrary_FilterByCategory(t *testing.T) {
	srv, client := setupTestServer(t)
	registerAndLogin(t, srv, client, "filter@example.com", "Filter User", "password123")

	resp, err := client.Get(srv.URL + "/api/stitches?category=post")
	if err != nil {
		t.Fatalf("GET /api/stitches?category=post: %v", err)
	}
	var libResp map[string]any
	parseJSON(t, resp, &libResp)

	predefined := libResp["predefined"].([]any)
	for _, s := range predefined {
		st := s.(map[string]any)
		if st["category"] != "post" {
			t.Fatalf("filter by post should only return post stitches, got category %s", st["category"])
		}
	}
	if len(predefined) == 0 {
		t.Fatal("should have at least one post stitch")
	}
}

func TestIntegration_StitchLibrary_ListAll(t *testing.T) {
	srv, client := setupTestServer(t)
	registerAndLogin(t, srv, client, "listall@example.com", "ListAll User", "password123")

	resp, err := client.Get(srv.URL + "/api/stitches/all")
	if err != nil {
		t.Fatalf("GET /api/stitches/all: %v", err)
	}
	var allResp map[string]any
	parseJSON(t, resp, &allResp)

	stitches := allResp["stitches"].([]any)
	if len(stitches) == 0 {
		t.Fatal("should have at least predefined stitches")
	}
}

func TestIntegration_Pattern_CreateViewDelete(t *testing.T) {
	srv, client := setupTestServer(t)
	registerAndLogin(t, srv, client, "pattern@example.com", "Pattern User", "password123")

	// Get sc stitch ID.
	resp, _ := client.Get(srv.URL + "/api/stitches/all")
	var allResp map[string]any
	parseJSON(t, resp, &allResp)
	scID := findStitchID(t, allResp["stitches"].([]any), "sc")

	// 1. List patterns (should be empty).
	resp, _ = client.Get(srv.URL + "/api/patterns")
	var listResp map[string]any
	parseJSON(t, resp, &listResp)
	patterns := listResp["patterns"].([]any)
	if len(patterns) != 0 {
		t.Fatalf("expected 0 patterns, got %d", len(patterns))
	}

	// 2. Create a pattern.
	resp, err := client.Post(srv.URL+"/api/patterns", "application/json",
		jsonBody(map[string]any{
			"name":        "Test Amigurumi",
			"description": "A small amigurumi ball",
			"patternType": "round",
			"hookSize":    "4.0mm",
			"yarnWeight":  "Worsted",
			"instructionGroups": []map[string]any{
				{
					"label":       "Round 1",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": scID, "count": 6, "repeatCount": 1},
					},
				},
			},
		}))
	if err != nil {
		t.Fatalf("POST /api/patterns: %v", err)
	}
	var createResp map[string]any
	parseJSON(t, resp, &createResp)
	patternObj := createResp["pattern"].(map[string]any)
	patternID := strconv.FormatInt(int64(patternObj["id"].(float64)), 10)
	if patternObj["name"] != "Test Amigurumi" {
		t.Fatalf("expected pattern name 'Test Amigurumi', got %v", patternObj["name"])
	}

	// 3. View the pattern.
	resp, _ = client.Get(srv.URL + "/api/patterns/" + patternID)
	var viewResp map[string]any
	parseJSON(t, resp, &viewResp)
	if viewResp["stitchCount"].(float64) != 6 {
		t.Fatalf("expected stitch count 6, got %v", viewResp["stitchCount"])
	}
	patternText := viewResp["patternText"].(string)
	if patternText == "" {
		t.Fatal("expected non-empty pattern text")
	}

	// 4. Delete the pattern.
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/patterns/"+patternID, nil)
	resp, _ = client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", resp.StatusCode)
	}

	// 5. Verify it's gone.
	resp, _ = client.Get(srv.URL + "/api/patterns")
	parseJSON(t, resp, &listResp)
	patterns = listResp["patterns"].([]any)
	if len(patterns) != 0 {
		t.Fatalf("expected 0 patterns after delete, got %d", len(patterns))
	}
}

func TestIntegration_Pattern_UpdateAndPreview(t *testing.T) {
	srv, client := setupTestServer(t)
	registerAndLogin(t, srv, client, "update@example.com", "Update User", "password123")

	resp, _ := client.Get(srv.URL + "/api/stitches/all")
	var allResp map[string]any
	parseJSON(t, resp, &allResp)
	scID := findStitchID(t, allResp["stitches"].([]any), "sc")
	incID := findStitchID(t, allResp["stitches"].([]any), "inc")

	// Create a pattern.
	resp, _ = client.Post(srv.URL+"/api/patterns", "application/json",
		jsonBody(map[string]any{
			"name":        "Update Test",
			"patternType": "round",
			"instructionGroups": []map[string]any{
				{
					"label":       "Round 1",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": scID, "count": 6, "repeatCount": 1},
					},
				},
			},
		}))
	var createResp map[string]any
	parseJSON(t, resp, &createResp)
	patternID := strconv.FormatInt(int64(createResp["pattern"].(map[string]any)["id"].(float64)), 10)

	// Update the pattern to add a second group.
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/patterns/"+patternID,
		jsonBody(map[string]any{
			"name":        "Updated Pattern",
			"patternType": "round",
			"instructionGroups": []map[string]any{
				{
					"label":       "Round 1",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": scID, "count": 6, "repeatCount": 1},
					},
				},
				{
					"label":       "Round 2",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": incID, "count": 1, "repeatCount": 6},
					},
				},
			},
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/patterns/%s: %v", patternID, err)
	}
	var updateResp map[string]any
	parseJSON(t, resp, &updateResp)
	updatedPattern := updateResp["pattern"].(map[string]any)
	if updatedPattern["name"] != "Updated Pattern" {
		t.Fatalf("expected 'Updated Pattern', got %v", updatedPattern["name"])
	}
	groups := updatedPattern["instructionGroups"].([]any)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// Preview the pattern.
	resp, _ = client.Post(srv.URL+"/api/patterns/preview", "application/json",
		jsonBody(map[string]any{
			"name":        "Preview Test",
			"patternType": "round",
			"instructionGroups": []map[string]any{
				{
					"label":       "Round 1",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": scID, "count": 6, "repeatCount": 1},
					},
				},
			},
		}))
	var previewResp map[string]any
	parseJSON(t, resp, &previewResp)
	if previewResp["text"] == "" {
		t.Fatal("preview should return non-empty text")
	}
	if previewResp["stitchCount"].(float64) != 6 {
		t.Fatalf("preview stitch count: expected 6, got %v", previewResp["stitchCount"])
	}
}

func TestIntegration_Pattern_Duplicate(t *testing.T) {
	srv, client := setupTestServer(t)
	registerAndLogin(t, srv, client, "dup-pattern@example.com", "Dup User", "password123")

	resp, _ := client.Get(srv.URL + "/api/stitches/all")
	var allResp map[string]any
	parseJSON(t, resp, &allResp)
	scID := findStitchID(t, allResp["stitches"].([]any), "sc")

	// Create a pattern.
	resp, _ = client.Post(srv.URL+"/api/patterns", "application/json",
		jsonBody(map[string]any{
			"name":        "Original Pattern",
			"patternType": "round",
			"instructionGroups": []map[string]any{
				{
					"label":       "Round 1",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": scID, "count": 6, "repeatCount": 1},
					},
				},
			},
		}))
	var createResp map[string]any
	parseJSON(t, resp, &createResp)
	patternID := strconv.FormatInt(int64(createResp["pattern"].(map[string]any)["id"].(float64)), 10)

	// Duplicate.
	resp, err := client.Post(srv.URL+"/api/patterns/"+patternID+"/duplicate", "application/json", nil)
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	var dupResp map[string]any
	parseJSON(t, resp, &dupResp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("duplicate: expected 201, got %d", resp.StatusCode)
	}

	// Verify both appear in list.
	resp, _ = client.Get(srv.URL + "/api/patterns")
	var listResp map[string]any
	parseJSON(t, resp, &listResp)
	patterns := listResp["patterns"].([]any)
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(patterns))
	}
}

func TestIntegration_Pattern_Unauthenticated(t *testing.T) {
	srv, client := setupTestServer(t)

	resp, err := client.Get(srv.URL + "/api/patterns")
	if err != nil {
		t.Fatalf("GET /api/patterns: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestIntegration_WorkSession_NavigateToCompletion(t *testing.T) {
	srv, client := setupTestServer(t)
	registerAndLogin(t, srv, client, "session@example.com", "Session User", "password123")

	resp, _ := client.Get(srv.URL + "/api/stitches/all")
	var allResp map[string]any
	parseJSON(t, resp, &allResp)
	scID := findStitchID(t, allResp["stitches"].([]any), "sc")

	// Create a small pattern: Round 1: 3 sc.
	resp, _ = client.Post(srv.URL+"/api/patterns", "application/json",
		jsonBody(map[string]any{
			"name":        "Session Test",
			"patternType": "round",
			"instructionGroups": []map[string]any{
				{
					"label":       "Round 1",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": scID, "count": 3, "repeatCount": 1},
					},
				},
			},
		}))
	var createResp map[string]any
	parseJSON(t, resp, &createResp)
	patternID := strconv.FormatInt(int64(createResp["pattern"].(map[string]any)["id"].(float64)), 10)

	// Start a work session.
	resp, err := client.Post(srv.URL+"/api/patterns/"+patternID+"/sessions", "application/json", nil)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	var sessionResp map[string]any
	parseJSON(t, resp, &sessionResp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("start session: expected 201, got %d", resp.StatusCode)
	}
	sessionID := strconv.FormatInt(int64(sessionResp["session"].(map[string]any)["id"].(float64)), 10)

	// View the session.
	resp, _ = client.Get(srv.URL + "/api/sessions/" + sessionID)
	var viewResp map[string]any
	parseJSON(t, resp, &viewResp)
	progress := viewResp["progress"].(map[string]any)
	if progress["currentAbbr"] != "sc" {
		t.Fatalf("expected current stitch 'sc', got %v", progress["currentAbbr"])
	}

	// Navigate forward 3 times.
	for i := range 3 {
		resp, err = client.Post(srv.URL+"/api/sessions/"+sessionID+"/next", "application/json", nil)
		if err != nil {
			t.Fatalf("next %d: %v", i+1, err)
		}
		parseJSON(t, resp, &viewResp)
	}

	// Session should be completed.
	session := viewResp["session"].(map[string]any)
	if session["status"] != "completed" {
		t.Fatalf("expected status 'completed', got %v", session["status"])
	}
}

func TestIntegration_WorkSession_MultiGroupNavigateToCompletion(t *testing.T) {
	srv, client := setupTestServer(t)
	registerAndLogin(t, srv, client, "multigroup@example.com", "MultiGroup User", "password123")

	resp, _ := client.Get(srv.URL + "/api/stitches/all")
	var allResp map[string]any
	parseJSON(t, resp, &allResp)
	scID := findStitchID(t, allResp["stitches"].([]any), "sc")
	dcID := findStitchID(t, allResp["stitches"].([]any), "dc")

	// Create a 2-group pattern: 3x sc + 3x dc = 6 total stitches.
	resp, _ = client.Post(srv.URL+"/api/patterns", "application/json",
		jsonBody(map[string]any{
			"name":        "Multi-Group Test",
			"patternType": "round",
			"instructionGroups": []map[string]any{
				{
					"label":       "Round 1",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": scID, "count": 3, "repeatCount": 1},
					},
				},
				{
					"label":       "Round 2",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": dcID, "count": 3, "repeatCount": 1},
					},
				},
			},
		}))
	var createResp map[string]any
	parseJSON(t, resp, &createResp)
	patternID := strconv.FormatInt(int64(createResp["pattern"].(map[string]any)["id"].(float64)), 10)

	// Start session.
	resp, _ = client.Post(srv.URL+"/api/patterns/"+patternID+"/sessions", "application/json", nil)
	var sessionResp map[string]any
	parseJSON(t, resp, &sessionResp)
	sessionID := strconv.FormatInt(int64(sessionResp["session"].(map[string]any)["id"].(float64)), 10)

	// Navigate forward 3 times (complete group 0).
	var viewResp map[string]any
	for i := range 3 {
		resp, err := client.Post(srv.URL+"/api/sessions/"+sessionID+"/next", "application/json", nil)
		if err != nil {
			t.Fatalf("next %d: %v", i+1, err)
		}
		parseJSON(t, resp, &viewResp)
	}

	// Should still be active, in Round 2.
	session := viewResp["session"].(map[string]any)
	if session["status"] == "completed" {
		t.Fatal("session should NOT be completed after finishing only group 0")
	}
	progress := viewResp["progress"].(map[string]any)
	if progress["groupLabel"] != "Round 2" {
		t.Fatalf("expected 'Round 2', got %v", progress["groupLabel"])
	}

	// Navigate forward 3 more times (complete group 1).
	for i := range 3 {
		resp, err := client.Post(srv.URL+"/api/sessions/"+sessionID+"/next", "application/json", nil)
		if err != nil {
			t.Fatalf("next %d: %v", i+1, err)
		}
		parseJSON(t, resp, &viewResp)
	}

	session = viewResp["session"].(map[string]any)
	if session["status"] != "completed" {
		t.Fatalf("expected status 'completed', got %v", session["status"])
	}
}

func TestIntegration_WorkSession_NavigateBackward(t *testing.T) {
	srv, client := setupTestServer(t)
	registerAndLogin(t, srv, client, "backward@example.com", "Backward User", "password123")

	resp, _ := client.Get(srv.URL + "/api/stitches/all")
	var allResp map[string]any
	parseJSON(t, resp, &allResp)
	scID := findStitchID(t, allResp["stitches"].([]any), "sc")

	// Create pattern: 4 sc.
	resp, _ = client.Post(srv.URL+"/api/patterns", "application/json",
		jsonBody(map[string]any{
			"name":        "Backward Test",
			"patternType": "round",
			"instructionGroups": []map[string]any{
				{
					"label":       "Round 1",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": scID, "count": 4, "repeatCount": 1},
					},
				},
			},
		}))
	var createResp map[string]any
	parseJSON(t, resp, &createResp)
	patternID := strconv.FormatInt(int64(createResp["pattern"].(map[string]any)["id"].(float64)), 10)

	// Start session.
	resp, _ = client.Post(srv.URL+"/api/patterns/"+patternID+"/sessions", "application/json", nil)
	var sessionResp map[string]any
	parseJSON(t, resp, &sessionResp)
	sessionID := strconv.FormatInt(int64(sessionResp["session"].(map[string]any)["id"].(float64)), 10)

	// Forward 2 times.
	var viewResp map[string]any
	for range 2 {
		resp, _ = client.Post(srv.URL+"/api/sessions/"+sessionID+"/next", "application/json", nil)
		parseJSON(t, resp, &viewResp)
	}

	// Backward 1 time.
	resp, err := client.Post(srv.URL+"/api/sessions/"+sessionID+"/prev", "application/json", nil)
	if err != nil {
		t.Fatalf("prev: %v", err)
	}
	parseJSON(t, resp, &viewResp)

	// Should be at stitch count position 1 (0-based), so 1/4 completed.
	progress := viewResp["progress"].(map[string]any)
	if progress["completedStitches"].(float64) != 1 {
		t.Fatalf("expected 1 completed stitch after forward 2 then back 1, got %v", progress["completedStitches"])
	}
	session := viewResp["session"].(map[string]any)
	if session["status"] == "completed" {
		t.Fatal("session should not be completed after navigating backward")
	}
}

func TestIntegration_WorkSession_PauseResume(t *testing.T) {
	srv, client := setupTestServer(t)
	registerAndLogin(t, srv, client, "pause@example.com", "Pause User", "password123")

	resp, _ := client.Get(srv.URL + "/api/stitches/all")
	var allResp map[string]any
	parseJSON(t, resp, &allResp)
	scID := findStitchID(t, allResp["stitches"].([]any), "sc")

	resp, _ = client.Post(srv.URL+"/api/patterns", "application/json",
		jsonBody(map[string]any{
			"name":        "Pause Test",
			"patternType": "round",
			"instructionGroups": []map[string]any{
				{
					"label":       "Round 1",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": scID, "count": 4, "repeatCount": 1},
					},
				},
			},
		}))
	var createResp map[string]any
	parseJSON(t, resp, &createResp)
	patternID := strconv.FormatInt(int64(createResp["pattern"].(map[string]any)["id"].(float64)), 10)

	resp, _ = client.Post(srv.URL+"/api/patterns/"+patternID+"/sessions", "application/json", nil)
	var sessionResp map[string]any
	parseJSON(t, resp, &sessionResp)
	sessionID := strconv.FormatInt(int64(sessionResp["session"].(map[string]any)["id"].(float64)), 10)

	// Pause.
	resp, _ = client.Post(srv.URL+"/api/sessions/"+sessionID+"/pause", "application/json", nil)
	var viewResp map[string]any
	parseJSON(t, resp, &viewResp)
	if viewResp["session"].(map[string]any)["status"] != "paused" {
		t.Fatalf("expected paused, got %v", viewResp["session"].(map[string]any)["status"])
	}

	// Resume.
	resp, _ = client.Post(srv.URL+"/api/sessions/"+sessionID+"/resume", "application/json", nil)
	parseJSON(t, resp, &viewResp)
	if viewResp["session"].(map[string]any)["status"] != "active" {
		t.Fatalf("expected active, got %v", viewResp["session"].(map[string]any)["status"])
	}
}

func TestIntegration_WorkSession_Abandon(t *testing.T) {
	srv, client := setupTestServer(t)
	registerAndLogin(t, srv, client, "abandon@example.com", "Abandon User", "password123")

	resp, _ := client.Get(srv.URL + "/api/stitches/all")
	var allResp map[string]any
	parseJSON(t, resp, &allResp)
	scID := findStitchID(t, allResp["stitches"].([]any), "sc")

	resp, _ = client.Post(srv.URL+"/api/patterns", "application/json",
		jsonBody(map[string]any{
			"name":        "Abandon Test",
			"patternType": "round",
			"instructionGroups": []map[string]any{
				{
					"label":       "Round 1",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": scID, "count": 4, "repeatCount": 1},
					},
				},
			},
		}))
	var createResp map[string]any
	parseJSON(t, resp, &createResp)
	patternID := strconv.FormatInt(int64(createResp["pattern"].(map[string]any)["id"].(float64)), 10)

	resp, _ = client.Post(srv.URL+"/api/patterns/"+patternID+"/sessions", "application/json", nil)
	var sessionResp map[string]any
	parseJSON(t, resp, &sessionResp)
	sessionID := strconv.FormatInt(int64(sessionResp["session"].(map[string]any)["id"].(float64)), 10)

	// Abandon (delete).
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/sessions/"+sessionID, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("DELETE session: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("abandon: expected 204, got %d", resp.StatusCode)
	}

	// Session should be gone.
	resp, _ = client.Get(srv.URL + "/api/sessions/" + sessionID)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("after abandon: expected 404, got %d", resp.StatusCode)
	}
}

func TestIntegration_ImageUploadAndServe(t *testing.T) {
	srv, client := setupTestServer(t)
	registerAndLogin(t, srv, client, "img@example.com", "Image User", "password123")

	resp, _ := client.Get(srv.URL + "/api/stitches/all")
	var allResp map[string]any
	parseJSON(t, resp, &allResp)
	scID := findStitchID(t, allResp["stitches"].([]any), "sc")

	// Create a pattern.
	resp, _ = client.Post(srv.URL+"/api/patterns", "application/json",
		jsonBody(map[string]any{
			"name":        "Image Test Pattern",
			"patternType": "round",
			"instructionGroups": []map[string]any{
				{
					"label":       "Round 1",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": scID, "count": 6, "repeatCount": 1},
					},
				},
			},
		}))
	var createResp map[string]any
	parseJSON(t, resp, &createResp)
	patternID := strconv.FormatInt(int64(createResp["pattern"].(map[string]any)["id"].(float64)), 10)

	// 1. Upload a valid PNG image.
	pngData := createTestPNG()
	resp, err := uploadImage(client, srv.URL, patternID, "0", "test.png", "image/png", pngData)
	if err != nil {
		t.Fatalf("upload PNG: %v", err)
	}
	var uploadResp map[string]any
	parseJSON(t, resp, &uploadResp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload PNG: expected 201, got %d", resp.StatusCode)
	}
	imageID := strconv.FormatInt(int64(uploadResp["image"].(map[string]any)["id"].(float64)), 10)

	// 2. Serve the image.
	resp, err = client.Get(srv.URL + "/api/images/" + imageID)
	if err != nil {
		t.Fatalf("GET image: %v", err)
	}
	imgBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("serve image: expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "image/png" {
		t.Fatalf("expected image/png, got %s", ct)
	}
	if len(imgBody) == 0 {
		t.Fatal("image body should not be empty")
	}

	// 3. Upload JPEG.
	jpegData := createTestJPEG()
	resp, err = uploadImage(client, srv.URL, patternID, "0", "test.jpg", "image/jpeg", jpegData)
	if err != nil {
		t.Fatalf("upload JPEG: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload JPEG: expected 201, got %d", resp.StatusCode)
	}

	// 4. View pattern should include groupImages.
	resp, _ = client.Get(srv.URL + "/api/patterns/" + patternID)
	var viewResp map[string]any
	parseJSON(t, resp, &viewResp)
	groupImages := viewResp["groupImages"].(map[string]any)
	if len(groupImages) == 0 {
		t.Fatal("expected group images in pattern view")
	}

	// 5. Delete an image.
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/images/"+imageID, nil)
	resp, _ = client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete image: expected 204, got %d", resp.StatusCode)
	}

	// 6. Image should be gone.
	resp, _ = client.Get(srv.URL + "/api/images/" + imageID)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("after delete: expected 404, got %d", resp.StatusCode)
	}
}

func TestIntegration_ImageUploadLimits(t *testing.T) {
	srv, client := setupTestServer(t)
	registerAndLogin(t, srv, client, "limit@example.com", "Limit User", "password123")

	resp, _ := client.Get(srv.URL + "/api/stitches/all")
	var allResp map[string]any
	parseJSON(t, resp, &allResp)
	scID := findStitchID(t, allResp["stitches"].([]any), "sc")

	resp, _ = client.Post(srv.URL+"/api/patterns", "application/json",
		jsonBody(map[string]any{
			"name":        "Limit Test Pattern",
			"patternType": "round",
			"instructionGroups": []map[string]any{
				{
					"label":       "Round 1",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": scID, "count": 6, "repeatCount": 1},
					},
				},
			},
		}))
	var createResp map[string]any
	parseJSON(t, resp, &createResp)
	patternID := strconv.FormatInt(int64(createResp["pattern"].(map[string]any)["id"].(float64)), 10)

	// Upload 5 images.
	for i := range 5 {
		pngData := createTestPNG()
		resp, err := uploadImage(client, srv.URL, patternID, "0", "img"+strconv.Itoa(i)+".png", "image/png", pngData)
		if err != nil {
			t.Fatalf("upload %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("upload %d: expected 201, got %d", i, resp.StatusCode)
		}
	}

	// 6th should be rejected.
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

func TestFullHappyPath(t *testing.T) {
	srv, client := setupTestServer(t)
	registerAndLogin(t, srv, client, "happy@example.com", "Happy Crocheter", "password123")

	// Get stitch IDs.
	resp, _ := client.Get(srv.URL + "/api/stitches/all")
	var allResp map[string]any
	parseJSON(t, resp, &allResp)
	scID := findStitchID(t, allResp["stitches"].([]any), "sc")

	// Create custom stitch.
	resp, _ = client.Post(srv.URL+"/api/stitches", "application/json",
		jsonBody(map[string]string{
			"abbreviation": "msc",
			"name":         "Magic Single Crochet",
			"category":     "custom",
			"description":  "A magical stitch",
		}))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create custom stitch: expected 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Create pattern.
	resp, _ = client.Post(srv.URL+"/api/patterns", "application/json",
		jsonBody(map[string]any{
			"name":        "Happy Path Pattern",
			"patternType": "round",
			"hookSize":    "5.0mm",
			"yarnWeight":  "Worsted",
			"description": "A full happy path test pattern",
			"instructionGroups": []map[string]any{
				{
					"label":       "Round 1",
					"repeatCount": 1,
					"stitchEntries": []map[string]any{
						{"stitchId": scID, "count": 3, "repeatCount": 1},
					},
				},
			},
		}))
	var createResp map[string]any
	parseJSON(t, resp, &createResp)
	patternID := strconv.FormatInt(int64(createResp["pattern"].(map[string]any)["id"].(float64)), 10)

	// View pattern.
	resp, _ = client.Get(srv.URL + "/api/patterns/" + patternID)
	var viewResp map[string]any
	parseJSON(t, resp, &viewResp)
	if viewResp["patternText"] == "" {
		t.Fatal("expected non-empty pattern text")
	}

	// Start work session.
	resp, _ = client.Post(srv.URL+"/api/patterns/"+patternID+"/sessions", "application/json", nil)
	var sessionResp map[string]any
	parseJSON(t, resp, &sessionResp)
	sessionID := strconv.FormatInt(int64(sessionResp["session"].(map[string]any)["id"].(float64)), 10)

	// Navigate to completion.
	var navResp map[string]any
	for i := range 3 {
		resp, err := client.Post(srv.URL+"/api/sessions/"+sessionID+"/next", "application/json", nil)
		if err != nil {
			t.Fatalf("next %d: %v", i+1, err)
		}
		parseJSON(t, resp, &navResp)
	}

	session := navResp["session"].(map[string]any)
	if session["status"] != "completed" {
		t.Fatalf("expected completed, got %v", session["status"])
	}

	// Dashboard still accessible.
	resp, _ = client.Get(srv.URL + "/api/dashboard")
	var dashResp map[string]any
	parseJSON(t, resp, &dashResp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard: expected 200, got %d", resp.StatusCode)
	}
}

// findStitchID finds the stitch ID for the given abbreviation in a stitches list.
func findStitchID(t *testing.T, stitches []any, abbr string) float64 {
	t.Helper()
	for _, s := range stitches {
		st := s.(map[string]any)
		if st["abbreviation"] == abbr {
			return st["id"].(float64)
		}
	}
	t.Fatalf("stitch '%s' not found", abbr)
	return 0
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

	req, err := http.NewRequest("POST", baseURL+"/api/patterns/"+patternID+"/groups/"+groupIndex+"/images", &buf)
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
