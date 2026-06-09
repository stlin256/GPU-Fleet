package server

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if csp := rec.Header().Get("Content-Security-Policy"); strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("web/dist responses should not allow inline scripts, got CSP %q", csp)
	}

	req = httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.URL.Path = "/../secret.txt"
	rec = httptest.NewRecorder()
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
		"下载诊断包",
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
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("fallback dashboard should keep inline script compatibility, got CSP %q", csp)
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

func TestAdminWriteRequiresSameOrigin(t *testing.T) {
	root := t.TempDir()
	app := newTestApp(t, root, filepath.Join(root, "missing-web"))
	handler := app.Handler()
	cookie := loginCookie(t, handler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/guest", bytes.NewReader([]byte(`{"enabled":true}`)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected missing origin to be rejected, got %d with body %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/guest", bytes.NewReader([]byte(`{"enabled":true}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://evil.example")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected cross-origin request to be rejected, got %d with body %q", rec.Code, rec.Body.String())
	}

	doJSON(t, handler, http.MethodPost, "/api/v1/admin/guest", map[string]bool{"enabled": true}, cookie, http.StatusOK, nil)
}

func TestVersionAPIReadsRepoChangelog(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "CHANGELOG.md"), []byte(`# Changelog

## [0.1.7] - 2026-06-06

### Title / 标题

- zh-CN: 仓库变更记录
- en-US: Repository changelog

### Fixed / 修复

- zh-CN: 设置页读取最新仓库 changelog。
- en-US: Settings reads the latest repository changelog.
`), 0600); err != nil {
		t.Fatal(err)
	}
	app := newTestAppWithRepo(t, root, filepath.Join(root, "missing-web"), repoDir)
	handler := app.Handler()

	var info version.ReleaseInfo
	doJSON(t, handler, http.MethodGet, "/api/v1/version", nil, loginCookie(t, handler), http.StatusOK, &info)
	if len(info.Changelog) == 0 || info.Changelog[0].Title != "仓库变更记录" || info.Changelog[0].TitleEN != "Repository changelog" {
		t.Fatalf("expected repo changelog in version response, got %+v", info.Changelog)
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

func TestGitFailureMessageKeepsDiagnosticDetail(t *testing.T) {
	raw := "fetch --quiet --prune: fatal: unable to access 'https://github.com/stlin256/GPU-Fleet.git/': GnuTLS, handshake failed: The TLS connection was non-properly terminated."
	message := gitFailureMessage("fetch", raw, "")
	if !strings.Contains(message, "TLS") || !strings.Contains(message, "更新代理") {
		t.Fatalf("expected actionable TLS/proxy message, got %q", message)
	}
	status := updateStatus{Supported: true, Upstream: "origin/main", Failed: true, Message: message, Detail: raw}
	if updateMessage(status) != message || status.Detail != raw {
		t.Fatalf("expected friendly message and raw detail to be preserved, got %+v", status)
	}
}

func TestAutoUpdateConfigDefaultsOnAndCanBeDisabled(t *testing.T) {
	if !(ServiceConfig{}).AutoUpdateOn() {
		t.Fatal("missing auto update config should default to enabled")
	}
	disabled := false
	if (ServiceConfig{AutoUpdateEnabled: &disabled}).AutoUpdateOn() {
		t.Fatal("explicit false auto update config should disable checks")
	}
	root := t.TempDir()
	app := newTestApp(t, root, filepath.Join(root, "missing-web"))
	handler := app.Handler()
	cookie := loginCookie(t, handler)

	var response struct {
		Service serviceStatus `json:"service"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/server-config", map[string]bool{"auto_update_enabled": false}, cookie, http.StatusOK, &response)
	if response.Service.AutoUpdateEnabled {
		t.Fatalf("expected auto update to be disabled, got %+v", response.Service)
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/server-config", map[string]bool{"auto_update_enabled": true}, cookie, http.StatusOK, &response)
	if !response.Service.AutoUpdateEnabled {
		t.Fatalf("expected auto update to be enabled, got %+v", response.Service)
	}
}

func TestSameVersionChangelogSummaryKeepsOnlyNewLines(t *testing.T) {
	beforeRaw := `## [0.1.7] - 2026-06-08

### Changed / 变更

- zh-CN: 已有变更。
- en-US: Existing change.
`
	afterRaw := `## [0.1.7] - 2026-06-08

### Changed / 变更

- zh-CN: 已有变更。
- en-US: Existing change.
- zh-CN: 新增自动更新通知。
- en-US: Added automatic update notices.
`
	before := version.ChangelogFromMarkdown(beforeRaw)
	after := version.ChangelogFromMarkdown(afterRaw)
	zh := newChangelogItems(changelogEntryItems(after[0], false), changelogEntryItems(before[0], false))
	en := newChangelogItems(changelogEntryItems(after[0], true), changelogEntryItems(before[0], true))
	if len(zh) != 1 || zh[0] != "新增自动更新通知。" {
		t.Fatalf("expected only the new Chinese line, got %+v", zh)
	}
	if len(en) != 1 || en[0] != "Added automatic update notices." {
		t.Fatalf("expected only the new English line, got %+v", en)
	}
	zh, en = updateSummaryFromChangelog(beforeRaw, afterRaw)
	if len(zh) != 1 || zh[0] != "新增自动更新通知。" || len(en) != 1 || en[0] != "Added automatic update notices." {
		t.Fatalf("expected changelog summary to keep changed lines, got zh=%+v en=%+v", zh, en)
	}
	zh, en = updateSummaryFallback(nil, nil)
	if zh[0] != "无更新说明" || en[0] != "No update notes." {
		t.Fatalf("expected no-notes fallback, got zh=%+v en=%+v", zh, en)
	}
	if updateMonitorInterval(true) != 30*time.Minute || updateMonitorInterval(false) != time.Hour {
		t.Fatalf("unexpected update monitor intervals")
	}
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
	if err := os.WriteFile(filepath.Join(source, "CHANGELOG.md"), []byte(`## [0.1.7] - 2026-06-08

### Changed / 变更

- zh-CN: 初始更新说明。
- en-US: Initial update note.
`), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, source, "add", "CHANGELOG.md", "cmd/gpufleet-server/main.go", "version.txt")
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
	if err := os.WriteFile(filepath.Join(source, "CHANGELOG.md"), []byte(`## [0.1.7] - 2026-06-08

### Changed / 变更

- zh-CN: 初始更新说明。
- en-US: Initial update note.
- zh-CN: 手动更新后显示变更内容。
- en-US: Manual updates show release notes after restart.
`), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, source, "add", "CHANGELOG.md", "version.txt")
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
	var noticeResponse struct {
		Notice *UpdateNotice `json:"notice"`
	}
	doJSON(t, handler, http.MethodGet, "/api/v1/admin/update/notice", nil, cookie, http.StatusOK, &noticeResponse)
	if noticeResponse.Notice == nil || noticeResponse.Notice.Kind != "update" {
		t.Fatalf("expected manual update notice, got %+v", noticeResponse.Notice)
	}
	if len(noticeResponse.Notice.Summary) != 1 || noticeResponse.Notice.Summary[0] != "手动更新后显示变更内容。" {
		t.Fatalf("expected manual update notice summary, got %+v", noticeResponse.Notice.Summary)
	}
	if len(noticeResponse.Notice.SummaryEN) != 1 || noticeResponse.Notice.SummaryEN[0] != "Manual updates show release notes after restart." {
		t.Fatalf("expected manual update notice English summary, got %+v", noticeResponse.Notice.SummaryEN)
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
	if err := os.WriteFile(filepath.Join(source, "CHANGELOG.md"), []byte(`## [0.1.7] - 2026-06-08

### Changed / 变更

- zh-CN: 初始二进制说明。
- en-US: Initial binary note.
`), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, source, "add", "CHANGELOG.md", "cmd/gpufleet-server/main.go", "internal/version/version.go")
	git(t, source, "commit", "-m", "initial")
	initialCommit := strings.TrimSpace(git(t, source, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(source, "CHANGELOG.md"), []byte(`## [0.1.7] - 2026-06-08

### Changed / 变更

- zh-CN: 初始二进制说明。
- en-US: Initial binary note.
- zh-CN: 二进制落后重建后显示说明。
- en-US: Rebuilds for stale binaries show update notes.
`), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, source, "add", "CHANGELOG.md")
	git(t, source, "commit", "-m", "update changelog")
	git(t, source, "remote", "add", "origin", remote)
	git(t, source, "push", "-u", "origin", "main")
	git(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	git(t, root, "clone", remote, local)

	oldCommit := version.Commit
	version.Commit = initialCommit
	defer func() { version.Commit = oldCommit }()

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
	if applied.Notice == nil || len(applied.Notice.Summary) != 1 || applied.Notice.Summary[0] != "二进制落后重建后显示说明。" {
		t.Fatalf("expected rebuild notice summary from running commit, got %+v", applied.Notice)
	}
	if applied.Notice.PreviousCommit != initialCommit {
		t.Fatalf("expected previous commit to use running commit %s, got %+v", initialCommit, applied.Notice)
	}
}

func TestUpdateSupplyChainStatusBlocksUntrustedNetworkRemote(t *testing.T) {
	status := updateStatus{
		Supported:    true,
		Upstream:     "origin/main",
		LocalCommit:  strings.Repeat("1", 40),
		RemoteCommit: strings.Repeat("2", 40),
		Available:    true,
	}
	trusted := updateSupplyChainStatusForRemote(status, "https://github.com/stlin256/GPU-Fleet.git")
	if !trusted.OK || trusted.Blocked || !trusted.RemoteTrusted || trusted.RemoteHost != "github.com" || trusted.RemoteRepository != "stlin256/gpu-fleet" {
		t.Fatalf("expected official HTTPS remote to pass, got %+v", trusted)
	}
	scp := updateSupplyChainStatusForRemote(status, "git@github.com:stlin256/GPU-Fleet.git")
	if !scp.OK || scp.Blocked || !scp.RemoteTrusted || scp.RemoteRepository != "stlin256/gpu-fleet" {
		t.Fatalf("expected official scp remote to pass, got %+v", scp)
	}
	local := updateSupplyChainStatusForRemote(status, filepath.Join(t.TempDir(), "remote.git"))
	if !local.OK || local.Blocked || local.RemoteKind != "local" {
		t.Fatalf("expected local test remote not to be blocked, got %+v", local)
	}
	if remoteNameFromUpstream("origin/main") != "origin" || remoteNameFromUpstream("upstream/release/0.1") != "upstream" || remoteNameFromUpstream("main") != "" {
		t.Fatalf("unexpected upstream remote parsing")
	}

	status.SupplyChain = updateSupplyChainStatusForRemote(status, "https://github.com/other/project.git")
	if !status.SupplyChain.Blocked || status.SupplyChain.OK {
		t.Fatalf("expected untrusted network remote to be blocked, got %+v", status.SupplyChain)
	}
	app := newTestApp(t, t.TempDir(), filepath.Join(t.TempDir(), "missing-web"))
	_, code, message := app.applyUpdateLockedWithStatus(context.Background(), false, status, time.Now().UTC())
	if code != http.StatusPreconditionFailed || !strings.Contains(message, "supply-chain") {
		t.Fatalf("expected supply-chain precondition failure, got code=%d message=%q", code, message)
	}
}

func TestUpdateSupplyChainStatusRequiresExactTargetCommit(t *testing.T) {
	status := updateStatus{
		Supported:    true,
		Upstream:     "origin/main",
		LocalCommit:  strings.Repeat("1", 40),
		RemoteCommit: "origin/main",
		Available:    true,
	}
	status.SupplyChain = updateSupplyChainStatusForRemote(status, "https://github.com/stlin256/GPU-Fleet.git")
	if !status.SupplyChain.Blocked || status.SupplyChain.ExactTargetCommit {
		t.Fatalf("expected non-exact update target to be blocked, got %+v", status.SupplyChain)
	}
	app := newTestApp(t, t.TempDir(), filepath.Join(t.TempDir(), "missing-web"))
	_, code, message := app.applyUpdateLockedWithStatus(context.Background(), false, status, time.Now().UTC())
	if code != http.StatusPreconditionFailed || !strings.Contains(message, "commit") {
		t.Fatalf("expected exact-target precondition failure, got code=%d message=%q", code, message)
	}
}

func TestBinaryOutdatedAcceptsShortCommitPrefix(t *testing.T) {
	fullCommit := "0123456789abcdef0123456789abcdef01234567"
	status := updateStatus{
		LocalCommit:    fullCommit,
		RunningCommit:  fullCommit[:7],
		RepoVersion:    "0.1.8",
		RunningVersion: "0.1.8",
	}
	if binaryOutdated(status) {
		t.Fatalf("short running commit prefix should match local commit: %+v", status)
	}
	status.RunningCommit = strings.ToUpper(fullCommit[:12])
	if binaryOutdated(status) {
		t.Fatalf("commit prefix matching should be case-insensitive: %+v", status)
	}
	status.RunningCommit = fullCommit[:6]
	if !binaryOutdated(status) {
		t.Fatalf("ambiguous commit prefixes shorter than 7 chars should not match: %+v", status)
	}
	status.RunningCommit = "fedcba9876543210fedcba9876543210fedcba98"
	if !binaryOutdated(status) {
		t.Fatalf("different running commit should be outdated: %+v", status)
	}
}

func TestRecentUpdateNoticeSuppressesRepeatedAutoRebuild(t *testing.T) {
	now := time.Now().UTC()
	fullCommit := "0123456789abcdef0123456789abcdef01234567"
	status := updateStatus{
		LocalCommit:    fullCommit,
		BinaryOutdated: true,
	}
	notice := &UpdateNotice{
		TargetCommit: fullCommit[:12],
		CompletedAt:  now.Add(-time.Minute),
	}
	if !automaticUpdateRecentlyCompletedForTarget(notice, status, now) {
		t.Fatalf("expected recent same-target update notice to suppress automatic rebuild")
	}
	status.Available = true
	if automaticUpdateRecentlyCompletedForTarget(notice, status, now) {
		t.Fatalf("remote updates should not be suppressed by a stale same-target rebuild notice")
	}
	status.Available = false
	notice.CompletedAt = now.Add(-(updateRestartGrace + time.Second))
	if automaticUpdateRecentlyCompletedForTarget(notice, status, now) {
		t.Fatalf("old update notices should not suppress automatic rebuild")
	}
	notice.CompletedAt = now.Add(-time.Minute)
	notice.TargetCommit = "fedcba9876543210fedcba9876543210fedcba98"
	if automaticUpdateRecentlyCompletedForTarget(notice, status, now) {
		t.Fatalf("different target commits should not suppress automatic rebuild")
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

	sampleAt := time.Now().UTC()
	util := 55.0
	if err := app.metrics.AppendBatch(model.SampleBatch{
		DeviceID:     created.Device.ID,
		AgentVersion: model.AgentVersion,
		Samples: []model.GPUSample{{
			Timestamp: sampleAt,
			GPUs: []model.GPUStatus{{
				GPUID:                 "0",
				UUIDHash:              "sensitive-uuid",
				Name:                  "NVIDIA Test GPU",
				DriverVersion:         "999.1",
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
		Timestamp: sampleAt,
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
	doJSON(t, handler, http.MethodGet, "/api/v1/guest/overview", nil, nil, http.StatusForbidden, nil)
	var guestConfig struct {
		OK      bool          `json:"ok"`
		Service serviceStatus `json:"service"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/guest", map[string]bool{"enabled": true}, cookie, http.StatusOK, &guestConfig)
	if !guestConfig.OK || !guestConfig.Service.GuestEnabled {
		t.Fatalf("expected guest access enabled, got %+v", guestConfig)
	}
	var guestOverview overviewResponse
	req := httptest.NewRequest(http.MethodGet, "/api/v1/guest/overview", nil)
	req.Header.Set("X-GPUFleet-Guest-Fingerprint", "fp-test")
	req.Header.Set("X-GPUFleet-Guest-Language", "zh-CN")
	req.Header.Set("X-GPUFleet-Guest-Platform", "test-platform")
	req.Header.Set("X-GPUFleet-Guest-Screen", "1920x1080x24")
	req.Header.Set("X-GPUFleet-Guest-Timezone", "Asia/Shanghai")
	req.RemoteAddr = "203.0.113.9:4567"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected guest overview 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &guestOverview); err != nil {
		t.Fatal(err)
	}
	if !guestOverview.Guest || len(guestOverview.LatestProcesses) != 0 || guestOverview.LatestGPUs[0].DeviceID == created.Device.ID || guestOverview.LatestGPUs[0].GPU.UUIDHash != "" || guestOverview.LatestGPUs[0].GPU.DriverVersion != "" {
		t.Fatalf("guest overview leaked sensitive data: %+v", guestOverview)
	}
	var guestSeries []SeriesPoint
	guestSeriesPath := "/api/v1/guest/gpus/0/series?device_id=" + url.QueryEscape(guestOverview.LatestGPUs[0].DeviceID) + "&hours=2"
	req = httptest.NewRequest(http.MethodGet, guestSeriesPath, nil)
	req.Header.Set("X-GPUFleet-Guest-Fingerprint", "fp-test")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected guest GPU series 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &guestSeries); err != nil {
		t.Fatal(err)
	}
	if len(guestSeries) == 0 {
		t.Fatalf("expected guest GPU series points, got none")
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/guest", map[string]bool{"enabled": false}, cookie, http.StatusOK, nil)
	doJSON(t, handler, http.MethodGet, "/api/v1/guest/overview", nil, nil, http.StatusForbidden, nil)
	doJSON(t, handler, http.MethodGet, guestSeriesPath, nil, nil, http.StatusForbidden, nil)
	var guestVisits struct {
		OK     bool         `json:"ok"`
		Visits []GuestVisit `json:"visits"`
	}
	doJSON(t, handler, http.MethodGet, "/api/v1/admin/guest/visits", nil, cookie, http.StatusOK, &guestVisits)
	if !guestVisits.OK || len(guestVisits.Visits) == 0 || guestVisits.Visits[0].Fingerprint != "fp-test" || guestVisits.Visits[0].Platform != "test-platform" {
		t.Fatalf("expected guest fingerprint visit, got %+v", guestVisits)
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

func TestInvalidAgentSignatureDoesNotConsumeNonce(t *testing.T) {
	root := t.TempDir()
	app := newTestApp(t, root, filepath.Join(root, "missing-web"))
	handler := app.Handler()
	cookie := loginCookie(t, handler)

	var created struct {
		Device deviceView `json:"device"`
		Secret string     `json:"secret"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/devices", map[string]string{"alias": "worker"}, cookie, http.StatusCreated, &created)

	heartbeat := model.Heartbeat{
		AgentVersion: model.AgentVersion,
		Hostname:     "test-host",
		OS:           "linux",
		GPUCount:     1,
		Timestamp:    time.Now().UTC(),
	}
	body, err := json.Marshal(heartbeat)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	nonce := "nonce-reused-after-bad-signature"
	path := "/api/v1/agent/heartbeat"
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.HeaderDeviceID, created.Device.ID)
	req.Header.Set(auth.HeaderTimestamp, at.Format(time.RFC3339))
	req.Header.Set(auth.HeaderNonce, nonce)
	req.Header.Set(auth.HeaderSignature, "bad-signature")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected bad signature to be unauthorized, got %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.HeaderDeviceID, created.Device.ID)
	req.Header.Set(auth.HeaderTimestamp, at.Format(time.RFC3339))
	req.Header.Set(auth.HeaderNonce, nonce)
	req.Header.Set(auth.HeaderSignature, auth.Sign(http.MethodPost, path, body, created.Device.ID, created.Secret, at, nonce))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected valid retry with same nonce to succeed, got %d: %s", rec.Code, rec.Body.String())
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

func TestPasswordHashUsesPBKDF2AndMigratesLegacyHashes(t *testing.T) {
	account, err := NewAdminAccount("admin", "admin-test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(account.PasswordHash, passwordHashPBKDF2Prefix+"$") {
		t.Fatalf("expected PBKDF2 password hash, got %q", account.PasswordHash)
	}
	if ok, upgrade := verifyPassword("admin-test", account); !ok || upgrade {
		t.Fatalf("expected PBKDF2 password to verify without upgrade, ok=%v upgrade=%v", ok, upgrade)
	}

	root := t.TempDir()
	store := &MetadataStore{
		path: filepath.Join(root, "metadata.json"),
		data: metadataFile{
			CreatedAt: time.Now().UTC(),
			Admin: AdminAccount{
				Username:     "admin",
				Salt:         "legacy-salt",
				Iterations:   120000,
				PasswordHash: deriveLegacyPassword("admin-test", "legacy-salt", 120000),
			},
			Devices:        map[string]*Device{},
			WebSessions:    map[string]WebSession{},
			LastProcessSet: map[string]time.Time{},
		},
	}
	if !store.VerifyAdmin("admin-test") {
		t.Fatal("expected legacy password hash to verify")
	}
	if !strings.HasPrefix(store.data.Admin.PasswordHash, passwordHashPBKDF2Prefix+"$") {
		t.Fatalf("expected legacy password hash to migrate, got %q", store.data.Admin.PasswordHash)
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
		Addr:                 "127.0.0.1:8088",
		AddrExplicit:         true,
		DataDir:              filepath.Join(root, "data"),
		WebDir:               filepath.Join(root, "missing-web"),
		MinFreeBytes:         1,
		Retention:            time.Hour,
		DisableUpdateMonitor: true,
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
		Addr:                 "127.0.0.1:8088",
		AddrExplicit:         true,
		DataDir:              filepath.Join(root, "data"),
		WebDir:               filepath.Join(root, "missing-web"),
		MinFreeBytes:         1,
		Retention:            time.Hour,
		AdminPassword:        "admin-test",
		DisableUpdateMonitor: true,
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
	var reserveUpdate struct {
		OK              bool          `json:"ok"`
		Service         serviceStatus `json:"service"`
		RestartRequired bool          `json:"restart_required"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/server-config", map[string]int{"min_free_mb": 128}, cookie, http.StatusOK, &reserveUpdate)
	if !reserveUpdate.OK || reserveUpdate.Service.MinFreeBytes != 128*1024*1024 || reserveUpdate.RestartRequired {
		t.Fatalf("unexpected disk reserve update response: %+v", reserveUpdate)
	}
	var reserveOverview overviewResponse
	doJSON(t, handler, http.MethodGet, "/api/v1/overview", nil, cookie, http.StatusOK, &reserveOverview)
	if reserveOverview.MinFreeSpaceBytes != 128*1024*1024 || reserveOverview.Disk.MinFreeBytes != 128*1024*1024 {
		t.Fatalf("disk reserve should update immediately, got overview=%d disk=%d", reserveOverview.MinFreeSpaceBytes, reserveOverview.Disk.MinFreeBytes)
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/server-config", map[string]int{"min_free_mb": 32}, cookie, http.StatusBadRequest, nil)
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
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/update/proxy", map[string]string{"proxy_url": "http://proxy-user:local-dev-secret@127.0.0.1:7890"}, cookie, http.StatusOK, &proxyUpdate)
	if !proxyUpdate.OK || proxyUpdate.Service.UpdateProxy != "http://proxy-user:local-dev-secret@127.0.0.1:7890" || proxyUpdate.RestartRequired {
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

	var manualRestart struct {
		OK              bool          `json:"ok"`
		Service         serviceStatus `json:"service"`
		RestartRequired bool          `json:"restart_required"`
		Restarting      bool          `json:"restarting"`
		RestartAt       time.Time     `json:"restart_at"`
	}
	doJSON(t, handler, http.MethodPost, "/api/v1/admin/restart", nil, cookie, http.StatusOK, &manualRestart)
	if !manualRestart.OK || !manualRestart.RestartRequired || !manualRestart.Restarting || manualRestart.RestartAt.IsZero() {
		t.Fatalf("unexpected manual restart response: %+v", manualRestart)
	}
	doJSON(t, handler, http.MethodGet, "/api/v1/admin/restart", nil, cookie, http.StatusMethodNotAllowed, nil)

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

	if err := app.meta.AddAuditEvent(AuditEvent{
		At:        time.Now().UTC(),
		Type:      "diagnostics_test",
		Message:   "captured remote address",
		RemoteIP:  "203.0.113.42",
		RequestID: "req-test",
	}); err != nil {
		t.Fatal(err)
	}
	app.updateMu.Lock()
	app.updateStatusCache = updateStatus{
		Supported: true,
		Remote:    "https://repo-user:local-dev-secret@example.com/stlin256/GPU-Fleet.git",
		CheckedAt: time.Now().UTC(),
		Message:   "cached update status",
		Detail:    "local-dev-secret",
	}
	app.updateStatusCacheOK = true
	app.updateMu.Unlock()
	diagnostics := downloadZipFileContents(t, handler, cookie, "/api/v1/admin/diagnostics/download")
	raw, ok := diagnostics["diagnostics.json"]
	if !ok {
		t.Fatalf("expected diagnostics.json in diagnostics download, got %+v", diagnostics)
	}
	reportText := string(raw)
	for _, forbidden := range []string{"local-dev-secret", "PRIVATE KEY", "203.0.113.42"} {
		if strings.Contains(reportText, forbidden) {
			t.Fatalf("diagnostics report leaked %q: %s", forbidden, reportText)
		}
	}
	var report diagnosticsReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if report.Product != version.Product || report.Version != version.Version || report.Runtime.GoVersion == "" || report.Storage.DatabaseSizeBytes == 0 || len(report.Devices) == 0 {
		t.Fatalf("unexpected diagnostics report summary: %+v", report)
	}
	if report.Service.UpdateProxy != "http://redacted:redacted@127.0.0.1:7890" {
		t.Fatalf("expected redacted update proxy, got %q", report.Service.UpdateProxy)
	}
	if report.Update == nil || report.Update.Remote != "https://redacted:redacted@example.com/stlin256/GPU-Fleet.git" {
		t.Fatalf("expected cached update status with redacted remote, got %+v", report.Update)
	}
	if len(report.Audit) == 0 || report.Audit[0].RemoteIP != "203.0.113.x" {
		t.Fatalf("expected masked audit remote IP, got %+v", report.Audit)
	}
}

func TestMetricsStoreCompactsColdSegmentsAndCleansBySegmentTime(t *testing.T) {
	root := t.TempDir()
	store, err := NewMetricsStore(filepath.Join(root, "metrics"), 1, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Hour)
	oldAt := now.Add(-8 * 24 * time.Hour)
	expiredAt := now.Add(-31 * 24 * time.Hour)
	util := 42.0
	power := 188.5
	batchFor := func(at time.Time) model.SampleBatch {
		return model.SampleBatch{
			DeviceID:     "rig-compact",
			AgentVersion: model.AgentVersion,
			Samples: []model.GPUSample{{
				Timestamp: at,
				GPUs: []model.GPUStatus{{
					GPUID:                 "0",
					UUIDHash:              "uuid-compact",
					Name:                  "NVIDIA Test GPU",
					DriverVersion:         "999.1",
					MemoryTotalBytes:      24 * 1024 * 1024 * 1024,
					MemoryUsedBytes:       6 * 1024 * 1024 * 1024,
					UtilizationGPUPercent: &util,
					PowerDrawWatts:        &power,
				}},
			}},
		}
	}

	if err := store.AppendBatch(batchFor(oldAt)); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendBatch(batchFor(oldAt.Add(10 * time.Minute))); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendBatch(batchFor(expiredAt)); err != nil {
		t.Fatal(err)
	}
	expiredPath := filepath.Join(root, "metrics", "samples-"+expiredAt.Format("2006010215")+".jsonl.gz")
	if err := os.Chtimes(expiredPath, now, now); err != nil {
		t.Fatal(err)
	}

	store.lastCleanup = now.Add(-2 * time.Hour)
	if err := store.AppendBatch(batchFor(now)); err != nil {
		t.Fatal(err)
	}

	oldPath := filepath.Join(root, "metrics", "samples-"+oldAt.Format("2006010215")+".jsonl.gz")
	file, err := os.Open(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	gr, err := gzip.NewReader(file)
	if err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if gr.Header.Comment != compactSegmentComment {
		_ = gr.Close()
		_ = file.Close()
		t.Fatalf("expected cold segment to be compacted, got comment %q", gr.Header.Comment)
	}
	_ = gr.Close()
	_ = file.Close()

	points, err := store.Series("rig-compact", "0", oldAt.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(points) < 3 {
		t.Fatalf("expected compacted data to remain readable, got %d points", len(points))
	}
	if _, err := os.Stat(expiredPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected expired segment to be deleted by segment time, got err=%v", err)
	}
	if days := store.StoredDays(); days < 8 || days > 10 {
		t.Fatalf("expected stored days to reflect metric segment span, got %d", days)
	}
}

func TestMetricsStoreSeriesRollupCoversThirtyDayBoundary(t *testing.T) {
	root := t.TempDir()
	store, err := NewMetricsStore(filepath.Join(root, "metrics"), 1, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	util := 61.0
	power := 244.0
	sampleAt := time.Now().UTC().Add(-hourRollupAge + 2*time.Minute)
	if err := store.AppendBatch(model.SampleBatch{
		DeviceID:     "rig-rollup",
		AgentVersion: model.AgentVersion,
		Samples: []model.GPUSample{{
			Timestamp: sampleAt,
			GPUs: []model.GPUStatus{{
				GPUID:                 "0",
				UUIDHash:              "uuid-rollup",
				Name:                  "NVIDIA Rollup GPU",
				DriverVersion:         "999.1",
				MemoryTotalBytes:      24 * 1024 * 1024 * 1024,
				MemoryUsedBytes:       8 * 1024 * 1024 * 1024,
				UtilizationGPUPercent: &util,
				PowerDrawWatts:        &power,
			}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	segmentPath := filepath.Join(root, "metrics", "samples-"+sampleAt.Format("2006010215")+".jsonl.gz")
	if err := os.Remove(segmentPath); err != nil {
		t.Fatal(err)
	}

	points, err := store.SeriesRollup("rig-rollup", "0", time.Now().UTC().Add(-hourRollupAge))
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 30-day boundary query to use rollup index, got %d points", len(points))
	}
	if points[0].UtilizationGPUPercent == nil || *points[0].UtilizationGPUPercent != util {
		t.Fatalf("unexpected rollup point: %+v", points[0])
	}
}

func TestMetricsStoreStatsRollupCoversThirtyDayBoundary(t *testing.T) {
	root := t.TempDir()
	store, err := NewMetricsStore(filepath.Join(root, "metrics"), 1, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	util := 61.0
	power := 244.0
	sampleAt := time.Now().UTC().Add(-hourRollupAge + 2*time.Minute)
	if err := store.AppendBatch(model.SampleBatch{
		DeviceID:     "rig-rollup",
		AgentVersion: model.AgentVersion,
		Samples: []model.GPUSample{{
			Timestamp: sampleAt,
			GPUs: []model.GPUStatus{{
				GPUID:                 "0",
				UUIDHash:              "uuid-rollup",
				Name:                  "NVIDIA Rollup GPU",
				DriverVersion:         "999.1",
				MemoryTotalBytes:      24 * 1024 * 1024 * 1024,
				MemoryUsedBytes:       8 * 1024 * 1024 * 1024,
				UtilizationGPUPercent: &util,
				PowerDrawWatts:        &power,
			}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	segmentPath := filepath.Join(root, "metrics", "samples-"+sampleAt.Format("2006010215")+".jsonl.gz")
	if err := os.Remove(segmentPath); err != nil {
		t.Fatal(err)
	}

	stats, err := store.Stats("rig-rollup", time.Now().UTC().Add(-hourRollupAge))
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 30-day boundary stats to use rollup index, got %d rows", len(stats))
	}
	if stats[0].SampleCount != 1 || stats[0].AverageUtilizationPercent == nil || *stats[0].AverageUtilizationPercent != util {
		t.Fatalf("unexpected rollup stats: %+v", stats[0])
	}
}

func TestMetricsStoreSegmentLocksDoNotBlockUnrelatedWrites(t *testing.T) {
	root := t.TempDir()
	store, err := NewMetricsStore(filepath.Join(root, "metrics"), 1, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.lastCleanup = time.Now()
	store.mu.Unlock()

	oldSegment := time.Now().UTC().Add(-2 * time.Hour).Format("2006010215")
	readLock := store.segmentLockForName(oldSegment)
	readLock.RLock()
	defer readLock.RUnlock()

	done := make(chan error, 1)
	go func() {
		util := 33.0
		done <- store.AppendBatch(model.SampleBatch{
			DeviceID:     "rig-lock",
			AgentVersion: model.AgentVersion,
			Samples: []model.GPUSample{{
				Timestamp: time.Now().UTC(),
				GPUs: []model.GPUStatus{{
					GPUID:                 "0",
					Name:                  "NVIDIA Test GPU",
					UtilizationGPUPercent: &util,
				}},
			}},
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("append to a different segment was blocked by an unrelated segment read lock")
	}
}

func BenchmarkMetricsStoreThirtyDayStatsFromRollup(b *testing.B) {
	root := b.TempDir()
	store, err := NewMetricsStore(filepath.Join(root, "metrics"), 1, 30*24*time.Hour)
	if err != nil {
		b.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Hour)
	for deviceIndex := 0; deviceIndex < 20; deviceIndex++ {
		deviceID := "bench-device-" + strconv.Itoa(deviceIndex)
		batch := model.SampleBatch{
			DeviceID:     deviceID,
			AgentVersion: model.AgentVersion,
			Samples:      make([]model.GPUSample, 0, 30*24),
		}
		for hour := 0; hour < 30*24; hour++ {
			at := now.Add(-time.Duration(hour) * time.Hour)
			sample := model.GPUSample{
				Timestamp: at,
				GPUs:      make([]model.GPUStatus, 0, 4),
			}
			for gpuIndex := 0; gpuIndex < 4; gpuIndex++ {
				util := float64((deviceIndex + gpuIndex + hour) % 100)
				power := 120 + float64(gpuIndex*25)
				temp := 60 + float64(gpuIndex*3)
				sample.GPUs = append(sample.GPUs, model.GPUStatus{
					GPUID:                 "gpu" + strconv.Itoa(gpuIndex),
					Name:                  "Benchmark GPU",
					MemoryTotalBytes:      24 * 1024 * 1024 * 1024,
					MemoryUsedBytes:       uint64(4+gpuIndex) * 1024 * 1024 * 1024,
					UtilizationGPUPercent: &util,
					TemperatureCelsius:    &temp,
					PowerDrawWatts:        &power,
				})
			}
			batch.Samples = append(batch.Samples, sample)
		}
		if err := store.AppendBatch(batch); err != nil {
			b.Fatal(err)
		}
	}
	since := now.Add(-30 * 24 * time.Hour)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats, err := store.Stats("", since)
		if err != nil {
			b.Fatal(err)
		}
		if len(stats) != 80 {
			b.Fatalf("expected 80 GPU stat rows, got %d", len(stats))
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
		DataDir:              filepath.Join(root, "data"),
		WebDir:               webDir,
		RepoDir:              repoDir,
		MinFreeBytes:         1,
		Retention:            time.Hour,
		AdminPassword:        "admin-test",
		BootstrapDeviceID:    "local-dev",
		BootstrapSecret:      "local-dev-secret",
		DisableUpdateMonitor: true,
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
	contents := downloadZipFileContents(t, handler, cookie, "/api/v1/admin/database/download")
	entries := map[string]bool{}
	for name := range contents {
		entries[name] = true
	}
	return entries
}

func downloadZipFileContents(t *testing.T, handler http.Handler, cookie *http.Cookie, path string) map[string][]byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: expected 200, got %d with body %q", path, rec.Code, rec.Body.String())
	}
	reader, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	contents := map[string][]byte{}
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		contents[file.Name] = body
	}
	return contents
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
		if method == http.MethodPost || method == http.MethodPatch || method == http.MethodDelete || method == http.MethodPut {
			req.Header.Set("Origin", "http://example.com")
		}
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
