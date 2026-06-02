package server

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
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
	body := responseBody(t, app.Handler(), "/", http.StatusOK)
	for _, want := range []string{
		"GPUFleet",
		"服务设置",
		"settings-page",
		"服务状态",
		"密码更改",
		"端口配置",
		"HTTPS 证书",
		"数据库下载",
		"项目信息",
		"stlin256",
		"https://github.com/stlin256/GPUFleet",
		"配置引导",
		"gpu-trend-tile",
		"sparkline-wrap",
		"spark-tooltip",
		"offline-mask",
		"data-device-color",
		"deviceBorderColor",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("built-in dashboard should contain %q", want)
		}
	}
	if strings.Contains(body, "用户名") {
		t.Fatal("built-in dashboard should use password-only login")
	}
	for _, old := range []string{`class="meter"`, ".meter {"} {
		if strings.Contains(body, old) {
			t.Fatalf("built-in dashboard should not contain old meter UI %q", old)
		}
	}
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
		doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "wrong"}, nil, http.StatusUnauthorized, nil)
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "wrong"}, nil, http.StatusTooManyRequests, nil)
}

func TestLoginRejectsUsernameField(t *testing.T) {
	root := t.TempDir()
	app := newTestApp(t, root, filepath.Join(root, "missing-web"))
	handler := app.Handler()

	doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": "admin",
		"password": "admin-test",
	}, nil, http.StatusBadRequest, nil)
}

func TestLoginSessionRemembersBrowserForThirtyDays(t *testing.T) {
	root := t.TempDir()
	app := newTestApp(t, root, filepath.Join(root, "missing-web"))
	handler := app.Handler()
	rec := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "admin-test"}, nil, http.StatusOK, nil)
	cookie := sessionCookie(t, rec)
	if cookie.MaxAge != int(webSessionTTL.Seconds()) {
		t.Fatalf("expected 30 day session max age, got %d", cookie.MaxAge)
	}
	if time.Until(cookie.Expires) < 29*24*time.Hour {
		t.Fatalf("expected session cookie to expire in about 30 days, got %s", cookie.Expires)
	}

	restarted := newTestApp(t, root, filepath.Join(root, "missing-web"))
	restartedHandler := restarted.Handler()
	responseBodyWithCookie(t, restartedHandler, "/api/v1/overview", cookie, http.StatusOK)
}

func TestInitialSetupCreatesPasswordCredential(t *testing.T) {
	root := t.TempDir()
	app, generated, err := NewApp(Config{
		Addr:         "127.0.0.1:8088",
		AddrExplicit: true,
		DataDir:      filepath.Join(root, "data"),
		WebDir:       filepath.Join(root, "missing-web"),
		MinFreeBytes: 1,
		Retention:    time.Hour,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if generated != "" {
		t.Fatalf("first setup should not generate a hidden admin password, got %q", generated)
	}
	handler := app.Handler()

	var status setupStatusResponse
	doJSON(t, handler, http.MethodGet, "/api/v1/setup/status", nil, nil, http.StatusOK, &status)
	if !status.SetupRequired || status.SetupComplete {
		t.Fatalf("expected setup to be required, got %+v", status)
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "setup-pass"}, nil, http.StatusUnauthorized, nil)

	var applied struct {
		OK              bool          `json:"ok"`
		Service         serviceStatus `json:"service"`
		RestartRequired bool          `json:"restart_required"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/setup/apply", map[string]any{
		"password": "setup-pass",
		"port":     9091,
	}, nil, http.StatusOK, &applied)
	if !applied.OK || applied.Service.ConfiguredPort != 9091 || !applied.RestartRequired {
		t.Fatalf("unexpected setup apply response: %+v", applied)
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/setup/apply", map[string]any{
		"password": "another-pass",
		"port":     9092,
	}, nil, http.StatusForbidden, nil)
	rec := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "setup-pass"}, nil, http.StatusOK, nil)
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("expected password-only login to set a session cookie")
	}
}

func TestAdminRuntimeConfigCertificateAndDownload(t *testing.T) {
	root := t.TempDir()
	app := newTestApp(t, root, filepath.Join(root, "missing-web"))
	handler := app.Handler()
	cookie := loginCookie(t, handler)

	var portUpdate struct {
		OK              bool          `json:"ok"`
		Service         serviceStatus `json:"service"`
		RestartRequired bool          `json:"restart_required"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/server-config", map[string]int{"port": 9443}, cookie, http.StatusOK, &portUpdate)
	if !portUpdate.OK || portUpdate.Service.ConfiguredPort != 9443 || !portUpdate.RestartRequired {
		t.Fatalf("unexpected port update response: %+v", portUpdate)
	}

	certPEM, keyPEM, notAfter := testCertificate(t)
	var certUpdate struct {
		OK              bool          `json:"ok"`
		Service         serviceStatus `json:"service"`
		RestartRequired bool          `json:"restart_required"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/certificate", map[string]string{
		"certificate_pem": string(certPEM),
		"private_key_pem": string(keyPEM),
	}, cookie, http.StatusOK, &certUpdate)
	if !certUpdate.OK || !certUpdate.Service.HTTPSEnabled || certUpdate.Service.CertNotAfter.IsZero() || !certUpdate.RestartRequired {
		t.Fatalf("unexpected certificate update response: %+v", certUpdate)
	}
	if !sameSecond(certUpdate.Service.CertNotAfter, notAfter) {
		t.Fatalf("expected cert expiry %s, got %s", notAfter, certUpdate.Service.CertNotAfter)
	}

	doJSON(t, handler, http.MethodPost, "/api/v1/admin/password", map[string]string{
		"current_password": "admin-test",
		"next_password":    "next-pass",
	}, cookie, http.StatusOK, nil)
	doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "admin-test"}, nil, http.StatusUnauthorized, nil)
	doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "next-pass"}, nil, http.StatusOK, nil)

	entries := downloadZipEntries(t, handler, cookie)
	if !entries["metadata.json"] {
		t.Fatalf("expected metadata.json in database download, got %+v", entries)
	}
	for name := range entries {
		if strings.HasPrefix(name, "certs/") {
			t.Fatalf("database download must not include certificate private material, got %s", name)
		}
	}
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
	rec := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "admin-test"}, nil, http.StatusOK, nil)
	return sessionCookie(t, rec)
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	cookies := rec.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "gpufleet_session" {
			return cookie
		}
	}
	t.Fatal("missing session cookie")
	return nil
}

func downloadZipEntries(t *testing.T, handler http.Handler, cookie *http.Cookie) map[string]bool {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/database/download", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("database download: expected 200, got %d with body %q", rec.Code, rec.Body.String())
	}
	reader, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]bool{}
	for _, file := range reader.File {
		entries[file.Name] = true
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, rc)
		_ = rc.Close()
	}
	return entries
}

func testCertificate(t *testing.T) ([]byte, []byte, time.Time) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	notAfter := time.Now().UTC().Add(90 * 24 * time.Hour).Truncate(time.Second)
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "gpufleet.test"},
		NotBefore:    time.Now().UTC().Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return certPEM, keyPEM, notAfter
}

func sameSecond(left, right time.Time) bool {
	return left.UTC().Truncate(time.Second).Equal(right.UTC().Truncate(time.Second))
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
	body := responseBody(t, handler, path, wantStatus)
	if !strings.Contains(body, wantBody) {
		t.Fatalf("%s: expected response to contain %q, got %q", path, wantBody, body)
	}
}

func responseBody(t *testing.T, handler http.Handler, path string, wantStatus int) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("%s: expected status %d, got %d with body %q", path, wantStatus, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func responseBodyWithCookie(t *testing.T, handler http.Handler, path string, cookie *http.Cookie, wantStatus int) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("%s: expected status %d, got %d with body %q", path, wantStatus, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}
