package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
