package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gpufleet/internal/auth"
	"gpufleet/internal/model"
)

func TestStaticDashboardRouting(t *testing.T) {
	root := t.TempDir()
	webDir := filepath.Join(root, "web")
	assetDir := filepath.Join(webDir, "assets")
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<!doctype html><div id=\"root\">react-root</div>"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "app.js"), []byte("console.log('ok')"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}

	app := newTestApp(t, root, webDir)
	handler := app.Handler()

	assertStatusAndBody(t, handler, "/", http.StatusOK, "react-root")
	assertStatusAndBody(t, handler, "/assets/app.js", http.StatusOK, "console.log")
	assertStatusAndBody(t, handler, "/devices/local-dev", http.StatusOK, "react-root")
	assertStatusAndBody(t, handler, "/api/v1/unknown", http.StatusNotFound, `"not found"`)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.URL.Path = "/../secret.txt"
	rec := httptest.NewRecorder()
	if !app.serveWebDist(rec, req) {
		t.Fatal("expected static handler to handle traversal attempt")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected traversal attempt to return 404, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "outside") {
		t.Fatal("static handler served a file outside web dir")
	}
}

func TestBuiltInDashboardFallback(t *testing.T) {
	root := t.TempDir()
	app := newTestApp(t, root, filepath.Join(root, "missing-web"))
	assertStatusAndBody(t, app.Handler(), "/", http.StatusOK, "GPUFleet")
}

func TestAdminDeviceLifecycleAndAgentAuth(t *testing.T) {
	root := t.TempDir()
	app := newTestApp(t, root, filepath.Join(root, "missing-web"))
	handler := app.Handler()
	cookie := loginCookie(t, handler)

	var created struct {
		Device deviceView `json:"device"`
		Secret string     `json:"secret"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/devices", map[string]string{"alias": "worker"}, cookie, http.StatusCreated, &created)
	if created.Device.ID == "" || created.Secret == "" {
		t.Fatalf("expected created device and secret, got %+v", created)
	}
	assertSignedHeartbeat(t, handler, created.Device.ID, created.Secret, http.StatusAccepted)

	var disabled struct {
		Device deviceView `json:"device"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/devices/"+created.Device.ID+"/disable", nil, cookie, http.StatusOK, &disabled)
	if disabled.Device.Enabled {
		t.Fatal("expected disabled device")
	}
	assertSignedHeartbeat(t, handler, created.Device.ID, created.Secret, http.StatusForbidden)

	var enabled struct {
		Device deviceView `json:"device"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/devices/"+created.Device.ID+"/enable", nil, cookie, http.StatusOK, &enabled)
	if !enabled.Device.Enabled {
		t.Fatal("expected enabled device")
	}

	var rotated struct {
		Device deviceView `json:"device"`
		Secret string     `json:"secret"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/devices/"+created.Device.ID+"/rotate-secret", nil, cookie, http.StatusOK, &rotated)
	if rotated.Secret == "" || rotated.Secret == created.Secret {
		t.Fatal("expected a new rotated secret")
	}
	assertSignedHeartbeat(t, handler, created.Device.ID, created.Secret, http.StatusUnauthorized)
	assertSignedHeartbeat(t, handler, created.Device.ID, rotated.Secret, http.StatusAccepted)
}

func TestLoginRateLimit(t *testing.T) {
	root := t.TempDir()
	app := newTestApp(t, root, filepath.Join(root, "missing-web"))
	handler := app.Handler()

	for i := 0; i < 10; i++ {
		doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "admin", "password": "wrong"}, nil, http.StatusUnauthorized, nil)
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "admin", "password": "wrong"}, nil, http.StatusTooManyRequests, nil)
}

func newTestApp(t *testing.T, root, webDir string) *App {
	t.Helper()
	app, _, err := NewApp(Config{
		DataDir:           filepath.Join(root, "data"),
		WebDir:            webDir,
		MinFreeBytes:      1,
		Retention:         time.Hour,
		AdminPassword:     "admin-test",
		BootstrapDeviceID: "local-dev",
		BootstrapSecret:   "local-dev-secret",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func loginCookie(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	rec := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "admin", "password": "admin-test"}, nil, http.StatusOK, nil)
	cookies := rec.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "gpufleet_session" {
			return cookie
		}
	}
	t.Fatal("missing session cookie")
	return nil
}

func doJSON(t *testing.T, handler http.Handler, method, path string, body any, cookie *http.Cookie, wantStatus int, out any) *httptest.ResponseRecorder {
	t.Helper()
	raw := []byte("{}")
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("%s %s: expected status %d, got %d with body %q", method, path, wantStatus, rec.Code, rec.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("decode %s %s: %v", method, path, err)
		}
	}
	return rec
}

func assertSignedHeartbeat(t *testing.T, handler http.Handler, deviceID, secret string, wantStatus int) {
	t.Helper()
	heartbeat := model.Heartbeat{
		AgentVersion: model.AgentVersion,
		Hostname:     "test-host",
		OS:           "windows",
		GPUCount:     1,
		Timestamp:    time.Now().UTC(),
	}
	body, err := json.Marshal(heartbeat)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/heartbeat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if err := auth.AttachSignedHeaders(req, body, deviceID, secret, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("signed heartbeat: expected status %d, got %d with body %q", wantStatus, rec.Code, rec.Body.String())
	}
}

func assertStatusAndBody(t *testing.T, handler http.Handler, path string, wantStatus int, wantBody string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("%s: expected status %d, got %d with body %q", path, wantStatus, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), wantBody) {
		t.Fatalf("%s: expected response to contain %q, got %q", path, wantBody, rec.Body.String())
	}
}
