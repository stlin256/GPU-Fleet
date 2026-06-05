package server

import (
	"archive/zip"
	"bytes"
	"context"
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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gpufleet/internal/auth"
	"gpufleet/internal/model"
	"gpufleet/internal/version"
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
		"在线更新",
		"settings-update",
		"版本与变更",
		"/api/v1/version",
		"最近变更",
		"正在读取版本信息",
		"stlin256",
		"https://github.com/stlin256/GPU-Fleet",
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

func TestVersionAPIRequiresSession(t *testing.T) {
	root := t.TempDir()
	app := newTestApp(t, root, filepath.Join(root, "missing-web"))
	handler := app.Handler()

	doJSON(t, handler, http.MethodGet, "/api/v1/version", nil, nil, http.StatusUnauthorized, nil)

	var info version.ReleaseInfo
	doJSON(t, handler, http.MethodGet, "/api/v1/version", nil, loginCookie(t, handler), http.StatusOK, &info)
	if info.Product != version.Product || info.Version != version.Version || info.Author != "stlin256" || info.Repository != "https://github.com/stlin256/GPU-Fleet" {
		t.Fatalf("unexpected version response: %+v", info)
	}
	if len(info.Changelog) == 0 || info.Changelog[0].Version != info.Version || info.Changelog[0].Title == "" {
		t.Fatalf("expected current changelog entry in version response: %+v", info.Changelog)
	}
}

func TestUpdateAPIRequiresSessionAndHandlesUnsupportedRepo(t *testing.T) {
	root := t.TempDir()
	app := newTestAppWithRepo(t, root, filepath.Join(root, "missing-web"), filepath.Join(root, "not-a-repo"))
	handler := app.Handler()

	doJSON(t, handler, http.MethodGet, "/api/v1/admin/update/status", nil, nil, http.StatusUnauthorized, nil)
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/update/apply", nil, nil, http.StatusUnauthorized, nil)

	cookie := loginCookie(t, handler)
	var status updateStatus
	doJSON(t, handler, http.MethodGet, "/api/v1/admin/update/status", nil, cookie, http.StatusOK, &status)
	if status.Supported || status.Available || status.Message == "" {
		t.Fatalf("expected unsupported update status for non-repo dir, got %+v", status)
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/update/apply", nil, cookie, http.StatusBadRequest, nil)
}

func TestUpdateAPIReportsAndPullsFastForwardUpdates(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git executable is not available")
	}
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	source := filepath.Join(root, "source")
	local := filepath.Join(root, "local")

	git(t, root, "init", "--bare", remote)
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	git(t, source, "init")
	git(t, source, "checkout", "-b", "main")
	git(t, source, "config", "user.email", "test@example.com")
	git(t, source, "config", "user.name", "GPUFleet Test")
	if err := os.MkdirAll(filepath.Join(source, "cmd", "gpufleet-server"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "cmd", "gpufleet-server", "main.go"), []byte("package main\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "version.txt"), []byte("v1"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, source, "add", "cmd/gpufleet-server/main.go", "version.txt")
	git(t, source, "commit", "-m", "initial")
	git(t, source, "remote", "add", "origin", remote)
	git(t, source, "push", "-u", "origin", "main")
	git(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	git(t, root, "clone", remote, local)

	app := newTestAppWithRepo(t, root, filepath.Join(root, "missing-web"), local)
	var buildReq updateBuildRequest
	var restartReq updateRestartRequest
	var buildCalled bool
	var restartCalled bool
	app.updateBuildServer = func(ctx context.Context, req updateBuildRequest) (updateBuildResult, error) {
		buildCalled = true
		buildReq = req
		if err := os.WriteFile(req.OutputPath, []byte("test server binary"), 0700); err != nil {
			return updateBuildResult{}, err
		}
		return updateBuildResult{OutputPath: req.OutputPath, Output: "test build ok"}, nil
	}
	app.updateScheduleRestart = func(req updateRestartRequest) error {
		restartCalled = true
		restartReq = req
		_ = os.Remove(req.NextExe)
		return nil
	}
	app.updateExit = func() {}
	handler := app.Handler()
	cookie := loginCookie(t, handler)

	var initial updateStatus
	doJSON(t, handler, http.MethodGet, "/api/v1/admin/update/status", nil, cookie, http.StatusOK, &initial)
	if !initial.Supported || initial.Available || initial.Dirty || initial.Upstream == "" || initial.LocalCommit == "" || initial.RemoteCommit == "" {
		t.Fatalf("unexpected initial update status: %+v", initial)
	}

	if err := os.WriteFile(filepath.Join(source, "version.txt"), []byte("v2"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, source, "add", "version.txt")
	git(t, source, "commit", "-m", "update version")
	git(t, source, "push")

	var available updateStatus
	doJSON(t, handler, http.MethodGet, "/api/v1/admin/update/status", nil, cookie, http.StatusOK, &available)
	if !available.Supported || !available.Available || available.Behind != 1 || available.Ahead != 0 {
		t.Fatalf("expected one fast-forward update, got %+v", available)
	}

	var applied updateApplyResponse
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/update/apply", nil, cookie, http.StatusOK, &applied)
	if !applied.OK || applied.RestartRequired == false || !applied.Restarting || applied.RestartAt.IsZero() || applied.Status.Available || applied.Status.LocalCommit != applied.Status.RemoteCommit {
		t.Fatalf("unexpected update apply response: %+v", applied)
	}
	if !buildCalled || buildReq.RemoteCommit != available.RemoteCommit || buildReq.OutputPath == "" {
		t.Fatalf("expected update build hook to run for remote commit %s, got called=%v req=%+v", available.RemoteCommit, buildCalled, buildReq)
	}
	if !restartCalled || restartReq.CurrentExe == "" || restartReq.NextExe == "" || restartReq.PID <= 0 {
		t.Fatalf("expected update restart hook to run, got called=%v req=%+v", restartCalled, restartReq)
	}
	if !applied.DependencyStatus.OK || applied.DependencyStatus.Platform == "" {
		t.Fatalf("expected dependency status in update response, got %+v", applied.DependencyStatus)
	}
	raw, err := os.ReadFile(filepath.Join(local, "version.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "v2" {
		t.Fatalf("expected local checkout to be updated to v2, got %q", raw)
	}

	if err := os.WriteFile(filepath.Join(local, "dirty.txt"), []byte("dirty"), 0600); err != nil {
		t.Fatal(err)
	}
	var dirty updateStatus
	doJSON(t, handler, http.MethodGet, "/api/v1/admin/update/status", nil, cookie, http.StatusOK, &dirty)
	if !dirty.Dirty {
		t.Fatalf("expected dirty worktree to be reported, got %+v", dirty)
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/update/apply", nil, cookie, http.StatusConflict, nil)
}

func TestUpdateAPIRebuildsWhenRunningBinaryIsOutdated(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git executable is not available")
	}
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	source := filepath.Join(root, "source")
	local := filepath.Join(root, "local")

	git(t, root, "init", "--bare", remote)
	if err := os.MkdirAll(filepath.Join(source, "cmd", "gpufleet-server"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "internal", "version"), 0755); err != nil {
		t.Fatal(err)
	}
	git(t, source, "init")
	git(t, source, "checkout", "-b", "main")
	git(t, source, "config", "user.email", "test@example.com")
	git(t, source, "config", "user.name", "GPUFleet Test")
	if err := os.WriteFile(filepath.Join(source, "cmd", "gpufleet-server", "main.go"), []byte("package main\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "internal", "version", "version.go"), []byte("package version\n\nvar (\n\tVersion = \"9.9.9\"\n)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, source, "add", "cmd/gpufleet-server/main.go", "internal/version/version.go")
	git(t, source, "commit", "-m", "initial")
	git(t, source, "remote", "add", "origin", remote)
	git(t, source, "push", "-u", "origin", "main")
	git(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	git(t, root, "clone", remote, local)

	app := newTestAppWithRepo(t, root, filepath.Join(root, "missing-web"), local)
	var buildReq updateBuildRequest
	var restartReq updateRestartRequest
	app.updateBuildServer = func(ctx context.Context, req updateBuildRequest) (updateBuildResult, error) {
		buildReq = req
		if err := os.WriteFile(req.OutputPath, []byte("rebuilt binary"), 0700); err != nil {
			return updateBuildResult{}, err
		}
		return updateBuildResult{OutputPath: req.OutputPath, Output: "rebuild ok"}, nil
	}
	app.updateScheduleRestart = func(req updateRestartRequest) error {
		restartReq = req
		_ = os.Remove(req.NextExe)
		return nil
	}
	app.updateExit = func() {}
	handler := app.Handler()
	cookie := loginCookie(t, handler)

	var status updateStatus
	doJSON(t, handler, http.MethodGet, "/api/v1/admin/update/status", nil, cookie, http.StatusOK, &status)
	if !status.Supported || status.Available || !status.BinaryOutdated || status.RepoVersion != "9.9.9" {
		t.Fatalf("expected outdated binary status without remote update, got %+v", status)
	}

	var applied updateApplyResponse
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/update/apply", nil, cookie, http.StatusOK, &applied)
	if !applied.OK || !applied.RestartRequired || !applied.Restarting || applied.RestartAt.IsZero() {
		t.Fatalf("expected rebuild restart response, got %+v", applied)
	}
	if buildReq.RemoteCommit != status.LocalCommit {
		t.Fatalf("expected rebuild from local commit %q, got %+v", status.LocalCommit, buildReq)
	}
	if restartReq.CurrentExe == "" || restartReq.NextExe == "" || restartReq.PID <= 0 || !restartReq.ReplaceExecutable {
		t.Fatalf("expected executable replacement restart, got %+v", restartReq)
	}
}

func TestLinuxRestartScriptReplacesBinaryBeforeWaitingForOldProcess(t *testing.T) {
	req := updateRestartRequest{
		CurrentExe:        "/opt/gpufleet/gpufleet-server",
		NextExe:           "/opt/gpufleet/gpufleet-server.next",
		Args:              []string{"-addr", "0.0.0.0:9008"},
		WorkDir:           "/opt/gpufleet/repo",
		PID:               1234,
		RestartAt:         time.Now().UTC(),
		ReplaceExecutable: true,
	}
	script := linuxRestartScript(req, "/opt/gpufleet/gpufleet-update-restart.log", "/tmp/gpufleet-update.sh")
	moveIndex := strings.Index(script, "mv -f '/opt/gpufleet/gpufleet-server.next' '/opt/gpufleet/gpufleet-server'")
	waitIndex := strings.Index(script, "while kill -0 1234")
	if moveIndex < 0 || waitIndex < 0 || moveIndex > waitIndex {
		t.Fatalf("expected Linux restart script to replace binary before waiting for old process, got:\n%s", script)
	}
	if !strings.Contains(script, "already_running=0") || !strings.Contains(script, "/proc/[0-9]*/exe") {
		t.Fatalf("expected Linux restart script to avoid duplicate process starts, got:\n%s", script)
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

	var renamed struct {
		Device deviceView `json:"device"`
	}
	doJSON(t, handler, http.MethodPatch, "/api/v1/admin/devices/"+created.Device.ID, map[string]string{"alias": "worker-renamed"}, cookie, http.StatusOK, &renamed)
	if renamed.Device.Alias != "worker-renamed" {
		t.Fatalf("expected renamed device alias, got %+v", renamed)
	}
	assertSignedHeartbeat(t, handler, created.Device.ID, created.Secret, http.StatusAccepted)

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

	util := 55.0
	if err := app.metrics.AppendBatch(model.SampleBatch{
		DeviceID:     created.Device.ID,
		AgentVersion: model.AgentVersion,
		Samples: []model.GPUSample{{
			Timestamp: time.Now().UTC(),
			GPUs: []model.GPUStatus{{
				GPUID:                 "0",
				Name:                  "NVIDIA Test GPU",
				MemoryTotalBytes:      16 * 1024 * 1024 * 1024,
				MemoryUsedBytes:       4 * 1024 * 1024 * 1024,
				UtilizationGPUPercent: &util,
			}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.processes.Replace(model.ProcessBatch{
		DeviceID:  created.Device.ID,
		Timestamp: time.Now().UTC(),
		Processes: []model.ProcessSnapshot{{
			GPUID:           "0",
			PID:             1234,
			ProcessName:     "test.exe",
			UsedMemoryBytes: 1024,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	var beforeDelete overviewResponse
	doJSON(t, handler, http.MethodGet, "/api/v1/overview", nil, cookie, http.StatusOK, &beforeDelete)
	if beforeDelete.GPUCount != 1 || len(beforeDelete.LatestProcesses) != 1 || beforeDelete.DatabaseSizeBytes == 0 {
		t.Fatalf("expected created device snapshots before delete, got %+v", beforeDelete)
	}

	var deleted struct {
		Device deviceView `json:"device"`
	}
	doJSON(t, handler, http.MethodDelete, "/api/v1/admin/devices/"+created.Device.ID, nil, cookie, http.StatusOK, &deleted)
	if deleted.Device.ID != created.Device.ID {
		t.Fatalf("unexpected deleted device response: %+v", deleted)
	}
	assertSignedHeartbeat(t, handler, created.Device.ID, rotated.Secret, http.StatusUnauthorized)
	var afterDelete overviewResponse
	doJSON(t, handler, http.MethodGet, "/api/v1/overview", nil, cookie, http.StatusOK, &afterDelete)
	if afterDelete.DeviceCount != 1 || afterDelete.GPUCount != 0 || len(afterDelete.LatestProcesses) != 0 {
		t.Fatalf("expected deleted device to be removed from overview, got %+v", afterDelete)
	}
}

func TestLoginRateLimitShortWindow(t *testing.T) {
	root := t.TempDir()
	app := newTestApp(t, root, filepath.Join(root, "missing-web"))
	app.loginGuard = NewLoginGuard(100, 30*time.Minute, 5*time.Minute, time.Hour)
	handler := app.Handler()

	for i := 0; i < 10; i++ {
		doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "wrong"}, nil, http.StatusUnauthorized, nil)
	}
	rec := doJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "wrong"}, nil, http.StatusTooManyRequests, nil)
	assertRetryAfter(t, rec)
}

func TestLoginBruteForceLockout(t *testing.T) {
	root := t.TempDir()
	app := newTestApp(t, root, filepath.Join(root, "missing-web"))
	app.loginRate = NewRateLimiter(100, time.Minute)
	handler := app.Handler()

	for i := 0; i < 4; i++ {
		doJSONFrom(t, handler, "203.0.113.10:39000", http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "wrong"}, nil, http.StatusUnauthorized, nil)
	}
	rec := doJSONFrom(t, handler, "203.0.113.10:39000", http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "wrong"}, nil, http.StatusTooManyRequests, nil)
	assertRetryAfter(t, rec)

	doJSONFrom(t, handler, "203.0.113.10:39000", http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "admin-test"}, nil, http.StatusTooManyRequests, nil)
	doJSONFrom(t, handler, "203.0.113.11:39000", http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "admin-test"}, nil, http.StatusOK, nil)
}

func TestLoginSuccessClearsFailureState(t *testing.T) {
	root := t.TempDir()
	app := newTestApp(t, root, filepath.Join(root, "missing-web"))
	app.loginRate = NewRateLimiter(100, time.Minute)
	handler := app.Handler()

	for i := 0; i < 4; i++ {
		doJSONFrom(t, handler, "203.0.113.12:39000", http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "wrong"}, nil, http.StatusUnauthorized, nil)
	}
	doJSONFrom(t, handler, "203.0.113.12:39000", http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "admin-test"}, nil, http.StatusOK, nil)
	for i := 0; i < 4; i++ {
		doJSONFrom(t, handler, "203.0.113.12:39000", http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "wrong"}, nil, http.StatusUnauthorized, nil)
	}
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
	if status.Service.Language != defaultLanguage {
		t.Fatalf("expected default language %q, got %q", defaultLanguage, status.Service.Language)
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
		"language": "en-US",
	}, nil, http.StatusOK, &applied)
	if !applied.OK || applied.Service.ConfiguredPort != 9091 || applied.Service.Language != "en-US" || !applied.RestartRequired {
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

func TestServerDoesNotCreateBootstrapDeviceByDefault(t *testing.T) {
	root := t.TempDir()
	app, _, err := NewApp(Config{
		Addr:          "127.0.0.1:8088",
		AddrExplicit:  true,
		DataDir:       filepath.Join(root, "data"),
		WebDir:        filepath.Join(root, "missing-web"),
		MinFreeBytes:  1,
		Retention:     time.Hour,
		AdminPassword: "admin-test",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	handler := app.Handler()
	var overview overviewResponse
	doJSON(t, handler, http.MethodGet, "/api/v1/overview", nil, loginCookie(t, handler), http.StatusOK, &overview)
	if overview.DeviceCount != 0 || len(overview.Devices) != 0 {
		t.Fatalf("expected no bootstrap device without explicit bootstrap config, got %+v", overview.Devices)
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

	var languageUpdate struct {
		OK              bool          `json:"ok"`
		Service         serviceStatus `json:"service"`
		RestartRequired bool          `json:"restart_required"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/language", map[string]string{"language": "en-US"}, cookie, http.StatusOK, &languageUpdate)
	if !languageUpdate.OK || languageUpdate.Service.Language != "en-US" {
		t.Fatalf("unexpected language update response: %+v", languageUpdate)
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/language", map[string]string{"language": "fr-FR"}, cookie, http.StatusBadRequest, nil)

	var proxyUpdate struct {
		OK              bool          `json:"ok"`
		Service         serviceStatus `json:"service"`
		RestartRequired bool          `json:"restart_required"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/update/proxy", map[string]string{"proxy_url": "http://127.0.0.1:7890"}, cookie, http.StatusOK, &proxyUpdate)
	if !proxyUpdate.OK || proxyUpdate.Service.UpdateProxy != "http://127.0.0.1:7890" || proxyUpdate.RestartRequired {
		t.Fatalf("unexpected update proxy response: %+v", proxyUpdate)
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/update/proxy", map[string]string{"proxy_url": "ftp://127.0.0.1:21"}, cookie, http.StatusBadRequest, nil)

	app.updateScheduleRestart = func(req updateRestartRequest) error {
		if req.ReplaceExecutable || req.CurrentExe == "" || req.NextExe != "" || req.PID <= 0 || req.RestartAt.IsZero() {
			t.Fatalf("unexpected certificate restart request: %+v", req)
		}
		return nil
	}
	app.updateExit = func() {}

	certPEM, keyPEM, notAfter := testCertificate(t)
	var certUpdate struct {
		OK              bool          `json:"ok"`
		Service         serviceStatus `json:"service"`
		RestartRequired bool          `json:"restart_required"`
		Restarting      bool          `json:"restarting"`
		RestartAt       time.Time     `json:"restart_at"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/certificate", map[string]string{
		"certificate_pem": string(certPEM),
		"private_key_pem": string(keyPEM),
	}, cookie, http.StatusOK, &certUpdate)
	if !certUpdate.OK || !certUpdate.Service.HTTPSEnabled || certUpdate.Service.CertNotAfter.IsZero() || !certUpdate.RestartRequired || !certUpdate.Restarting || certUpdate.RestartAt.IsZero() {
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
	return newTestAppWithRepo(t, root, webDir, "")
}

func newTestAppWithRepo(t *testing.T, root, webDir, repoDir string) *App {
	t.Helper()
	app, _, err := NewApp(Config{
		DataDir:           filepath.Join(root, "data"),
		WebDir:            webDir,
		RepoDir:           repoDir,
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

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=Never")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s failed: %v\n%s", strings.Join(args, " "), dir, err, output)
	}
	return string(output)
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
	return doJSONFrom(t, handler, "", method, path, body, cookie, wantStatus, out)
}

func doJSONFrom(t *testing.T, handler http.Handler, remoteAddr, method, path string, body any, cookie *http.Cookie, wantStatus int, out any) *httptest.ResponseRecorder {
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
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
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

func assertRetryAfter(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Header().Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header, got headers %+v", rec.Header())
	}
	var body model.APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode retry body: %v", err)
	}
	if body.RetryAfterSeconds <= 0 {
		t.Fatalf("expected retry_after_seconds > 0, got %+v", body)
	}
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
