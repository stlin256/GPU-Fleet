package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	updateCheckTimeout   = 25 * time.Second
	updatePullTimeout    = 60 * time.Second
	updateBuildTimeout   = 3 * time.Minute
	updateRestartDelay   = 1200 * time.Millisecond
	updateOutputLimit    = 6000
	updateSourceEntry    = "./cmd/gpufleet-server"
	updateRestartLogName = "gpufleet-update-restart.log"
)

type updateStatus struct {
	Available    bool      `json:"available"`
	Supported    bool      `json:"supported"`
	Dirty        bool      `json:"dirty"`
	Branch       string    `json:"branch,omitempty"`
	Remote       string    `json:"remote,omitempty"`
	Upstream     string    `json:"upstream,omitempty"`
	LocalCommit  string    `json:"local_commit,omitempty"`
	RemoteCommit string    `json:"remote_commit,omitempty"`
	Behind       int       `json:"behind"`
	Ahead        int       `json:"ahead"`
	CheckedAt    time.Time `json:"checked_at"`
	Message      string    `json:"message,omitempty"`
}

type updateDependencyStatus struct {
	OK       bool     `json:"ok"`
	Platform string   `json:"platform"`
	Checked  []string `json:"checked,omitempty"`
	Missing  []string `json:"missing,omitempty"`
}

type updateApplyResponse struct {
	OK               bool                   `json:"ok"`
	Status           updateStatus           `json:"status"`
	Output           string                 `json:"output,omitempty"`
	BuildOutput      string                 `json:"build_output,omitempty"`
	DependencyStatus updateDependencyStatus `json:"dependency_status,omitempty"`
	RestartRequired  bool                   `json:"restart_required"`
	Restarting       bool                   `json:"restarting"`
	RestartAt        time.Time              `json:"restart_at,omitempty"`
}

type updateBuildRequest struct {
	RepoDir      string
	RemoteCommit string
	OutputPath   string
	ProxyURL     string
}

type updateBuildResult struct {
	OutputPath string
	Output     string
}

type updateRestartRequest struct {
	CurrentExe        string
	NextExe           string
	Args              []string
	WorkDir           string
	PID               int
	RestartAt         time.Time
	ReplaceExecutable bool
}

type updateBuildFunc func(context.Context, updateBuildRequest) (updateBuildResult, error)
type updateRestartFunc func(updateRestartRequest) error

func (a *App) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	a.updateMu.Lock()
	defer a.updateMu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), updateCheckTimeout)
	defer cancel()
	writeJSON(w, http.StatusOK, a.gitUpdateStatus(ctx))
}

func (a *App) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	a.updateMu.Lock()
	defer a.updateMu.Unlock()

	checkCtx, checkCancel := context.WithTimeout(r.Context(), updateCheckTimeout)
	status := a.gitUpdateStatus(checkCtx)
	checkCancel()
	if !status.Supported {
		writeError(w, http.StatusBadRequest, status.Message)
		return
	}
	if status.Upstream == "" {
		writeError(w, http.StatusBadRequest, "current branch has no upstream")
		return
	}
	if status.Dirty {
		writeError(w, http.StatusConflict, "server working tree has uncommitted changes")
		return
	}
	if status.Ahead > 0 {
		writeError(w, http.StatusConflict, "local branch is ahead of upstream; fast-forward update is not available")
		return
	}
	if !status.Available {
		writeJSON(w, http.StatusOK, updateApplyResponse{OK: true, Status: status, RestartRequired: false})
		return
	}

	deps := a.checkUpdateDependencies()
	if !deps.OK {
		_ = a.meta.AddAudit("server_update_failed", "missing update dependencies: "+strings.Join(deps.Missing, ", "))
		writeError(w, http.StatusPreconditionFailed, "missing update dependencies: "+strings.Join(deps.Missing, ", "))
		return
	}

	exePath, err := currentExecutablePath()
	if err != nil {
		_ = a.meta.AddAudit("server_update_failed", limitText(err.Error(), 300))
		writeError(w, http.StatusPreconditionFailed, err.Error())
		return
	}
	nextPath := exePath + ".next"
	_ = os.Remove(nextPath)

	buildCtx, buildCancel := context.WithTimeout(r.Context(), updateBuildTimeout)
	buildResult, err := a.updateBuildServer(buildCtx, updateBuildRequest{
		RepoDir:      a.config.RepoDir,
		RemoteCommit: status.RemoteCommit,
		OutputPath:   nextPath,
		ProxyURL:     a.meta.ServiceConfig().UpdateProxy,
	})
	buildCancel()
	if err != nil {
		_ = os.Remove(nextPath)
		_ = a.meta.AddAudit("server_update_failed", limitText(err.Error(), 300))
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("build update failed: %s", limitText(err.Error(), 500)))
		return
	}

	before := status.LocalCommit
	pullCtx, pullCancel := context.WithTimeout(r.Context(), updatePullTimeout)
	output, err := a.runGit(pullCtx, "pull", "--ff-only")
	pullCancel()
	if err != nil {
		_ = os.Remove(buildResult.OutputPath)
		_ = a.meta.AddAudit("server_update_failed", limitText(err.Error(), 300))
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("git pull failed: %s", limitText(err.Error(), 500)))
		return
	}

	finalCtx, finalCancel := context.WithTimeout(r.Context(), updateCheckTimeout)
	finalStatus := a.gitUpdateStatus(finalCtx)
	finalCancel()
	restartRequired := before != "" && finalStatus.LocalCommit != "" && before != finalStatus.LocalCommit
	if restartRequired {
		restartAt := time.Now().UTC().Add(updateRestartDelay)
		restartReq := updateRestartRequest{
			CurrentExe:        exePath,
			NextExe:           buildResult.OutputPath,
			Args:              append([]string(nil), os.Args[1:]...),
			WorkDir:           mustGetwd(),
			PID:               os.Getpid(),
			RestartAt:         restartAt,
			ReplaceExecutable: true,
		}
		if err := a.updateScheduleRestart(restartReq); err != nil {
			_ = os.Remove(buildResult.OutputPath)
			_ = a.meta.AddAudit("server_update_failed", limitText(err.Error(), 300))
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("schedule restart failed: %s", limitText(err.Error(), 500)))
			return
		}
		_ = a.meta.AddAudit("server_update_scheduled", fmt.Sprintf("pulled %s -> %s and scheduled automatic restart", shortCommit(before), shortCommit(finalStatus.LocalCommit)))
		go func() {
			time.Sleep(updateRestartDelay)
			if a.updateExit != nil {
				a.updateExit()
				return
			}
			os.Exit(0)
		}()
		writeJSON(w, http.StatusOK, updateApplyResponse{
			OK:               true,
			Status:           finalStatus,
			Output:           limitText(output, updateOutputLimit),
			BuildOutput:      limitText(buildResult.Output, updateOutputLimit),
			DependencyStatus: deps,
			RestartRequired:  true,
			Restarting:       true,
			RestartAt:        restartAt,
		})
		return
	}

	_ = os.Remove(buildResult.OutputPath)
	writeJSON(w, http.StatusOK, updateApplyResponse{
		OK:               true,
		Status:           finalStatus,
		Output:           limitText(output, updateOutputLimit),
		BuildOutput:      limitText(buildResult.Output, updateOutputLimit),
		DependencyStatus: deps,
		RestartRequired:  false,
		Restarting:       false,
	})
}

func (a *App) gitUpdateStatus(ctx context.Context) updateStatus {
	status := updateStatus{
		CheckedAt: time.Now().UTC(),
		Message:   "checking repository",
	}
	if _, err := a.repoDir(); err != nil {
		status.Message = err.Error()
		return status
	}

	inside, err := a.runGit(ctx, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		status.Message = "configured repo dir is not a Git working tree"
		return status
	}
	status.Supported = true

	if branch, err := a.runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		status.Branch = strings.TrimSpace(branch)
	}
	if remote, err := a.runGit(ctx, "remote", "get-url", "origin"); err == nil {
		status.Remote = sanitizeGitRemote(strings.TrimSpace(remote))
	}
	if local, err := a.runGit(ctx, "rev-parse", "HEAD"); err == nil {
		status.LocalCommit = strings.TrimSpace(local)
	}
	status.Dirty = a.workingTreeDirty(ctx)

	upstream, err := a.runGit(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		status.Message = "current branch has no upstream"
		return status
	}
	status.Upstream = strings.TrimSpace(upstream)

	if _, err := a.runGit(ctx, "fetch", "--quiet", "--prune"); err != nil {
		status.Message = "fetch failed: " + limitText(cleanCommandError(err), 500)
		return status
	}
	if remoteCommit, err := a.runGit(ctx, "rev-parse", "@{u}"); err == nil {
		status.RemoteCommit = strings.TrimSpace(remoteCommit)
	}
	if counts, err := a.runGit(ctx, "rev-list", "--left-right", "--count", "HEAD...@{u}"); err == nil {
		status.Ahead, status.Behind = parseAheadBehind(counts)
	}
	status.Available = status.Behind > 0
	status.Message = updateMessage(status)
	return status
}

func (a *App) workingTreeDirty(ctx context.Context) bool {
	out, err := a.runGit(ctx, "status", "--porcelain")
	return err == nil && strings.TrimSpace(out) != ""
}

func (a *App) repoDir() (string, error) {
	if strings.TrimSpace(a.config.RepoDir) == "" {
		return "", errors.New("repo dir is not configured")
	}
	abs, err := filepath.Abs(a.config.RepoDir)
	if err != nil {
		return "", fmt.Errorf("resolve repo dir: %w", err)
	}
	return abs, nil
}

func (a *App) runGit(ctx context.Context, args ...string) (string, error) {
	repoDir, err := a.repoDir()
	if err != nil {
		return "", err
	}
	return runGitInDir(ctx, repoDir, a.meta.ServiceConfig().UpdateProxy, args...)
}

func (a *App) checkUpdateDependencies() updateDependencyStatus {
	status := updateDependencyStatus{OK: true, Platform: runtime.GOOS}
	check := func(name string, err error) {
		if err == nil {
			status.Checked = append(status.Checked, name)
			return
		}
		status.OK = false
		status.Missing = append(status.Missing, name+" ("+err.Error()+")")
	}

	repoDir, err := a.repoDir()
	check("repo-dir", err)
	check("git", lookPath("git"))
	check("go", lookPath("go"))
	if err == nil {
		check(updateSourceEntry, fileExists(filepath.Join(repoDir, "cmd", "gpufleet-server", "main.go")))
	}
	exePath, exeErr := currentExecutablePath()
	check("current executable", exeErr)
	if exeErr == nil {
		check("executable directory writable", writableDir(filepath.Dir(exePath)))
	}
	switch runtime.GOOS {
	case "windows":
		check("powershell.exe", lookPath("powershell.exe"))
	case "linux":
		check("/bin/sh", fileExists("/bin/sh"))
	default:
		status.OK = false
		status.Missing = append(status.Missing, "unsupported platform "+runtime.GOOS)
	}
	return status
}

func defaultBuildServerForUpdate(ctx context.Context, req updateBuildRequest) (updateBuildResult, error) {
	if strings.TrimSpace(req.RemoteCommit) == "" {
		return updateBuildResult{}, errors.New("remote commit is empty")
	}
	repoDir, err := filepath.Abs(req.RepoDir)
	if err != nil {
		return updateBuildResult{}, fmt.Errorf("resolve repo dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(req.OutputPath), 0755); err != nil {
		return updateBuildResult{}, fmt.Errorf("prepare output dir: %w", err)
	}
	worktreeDir := filepath.Join(os.TempDir(), "gpufleet-update-worktree-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	defer os.RemoveAll(worktreeDir)

	if output, err := runGitInDir(ctx, repoDir, req.ProxyURL, "worktree", "add", "--detach", "--quiet", worktreeDir, req.RemoteCommit); err != nil {
		return updateBuildResult{}, fmt.Errorf("create update worktree: %s", limitText(output+err.Error(), 500))
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = runGitInDir(cleanupCtx, repoDir, req.ProxyURL, "worktree", "remove", "--force", worktreeDir)
	}()

	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", req.OutputPath, updateSourceEntry)
	cmd.Dir = worktreeDir
	cmd.Env = updateProxyEnv(append(os.Environ(), "GOWORK=off"), req.ProxyURL)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	combined := limitText(stdout.String()+stderr.String(), updateOutputLimit)
	if ctx.Err() != nil {
		return updateBuildResult{OutputPath: req.OutputPath, Output: combined}, ctx.Err()
	}
	if err != nil {
		return updateBuildResult{OutputPath: req.OutputPath, Output: combined}, fmt.Errorf("go build failed: %s", strings.TrimSpace(combined))
	}
	return updateBuildResult{OutputPath: req.OutputPath, Output: combined}, nil
}

func defaultScheduleRestartAfterUpdate(req updateRestartRequest) error {
	if req.PID <= 0 {
		return errors.New("invalid current process id")
	}
	if req.CurrentExe == "" {
		return errors.New("current executable path is empty")
	}
	if req.ReplaceExecutable {
		if req.NextExe == "" {
			return errors.New("new executable path is empty")
		}
		if _, err := os.Stat(req.NextExe); err != nil {
			return fmt.Errorf("new server executable is not available: %w", err)
		}
	}
	if req.RestartAt.IsZero() {
		req.RestartAt = time.Now().UTC().Add(updateRestartDelay)
	}
	switch runtime.GOOS {
	case "windows":
		return scheduleWindowsRestart(req)
	case "linux":
		return scheduleLinuxRestart(req)
	default:
		return fmt.Errorf("unsupported restart platform %s", runtime.GOOS)
	}
}

func scheduleWindowsRestart(req updateRestartRequest) error {
	helper, err := os.CreateTemp("", "gpufleet-update-*.ps1")
	if err != nil {
		return fmt.Errorf("create restart helper: %w", err)
	}
	helperPath := helper.Name()
	logPath := filepath.Join(filepath.Dir(req.CurrentExe), updateRestartLogName)
	script := windowsRestartScript(req, logPath, helperPath)
	if _, err := helper.WriteString(script); err != nil {
		helper.Close()
		return fmt.Errorf("write restart helper: %w", err)
	}
	if err := helper.Close(); err != nil {
		return fmt.Errorf("close restart helper: %w", err)
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", helperPath)
	cmd.Dir = req.WorkDir
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func scheduleLinuxRestart(req updateRestartRequest) error {
	helper, err := os.CreateTemp("", "gpufleet-update-*.sh")
	if err != nil {
		return fmt.Errorf("create restart helper: %w", err)
	}
	helperPath := helper.Name()
	logPath := filepath.Join(filepath.Dir(req.CurrentExe), updateRestartLogName)
	script := linuxRestartScript(req, logPath, helperPath)
	if _, err := helper.WriteString(script); err != nil {
		helper.Close()
		return fmt.Errorf("write restart helper: %w", err)
	}
	if err := helper.Close(); err != nil {
		return fmt.Errorf("close restart helper: %w", err)
	}
	if err := os.Chmod(helperPath, 0700); err != nil {
		return fmt.Errorf("chmod restart helper: %w", err)
	}
	cmd := exec.Command("/bin/sh", helperPath)
	cmd.Dir = req.WorkDir
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func windowsRestartScript(req updateRestartRequest, logPath, helperPath string) string {
	args := make([]string, 0, len(req.Args))
	for _, arg := range req.Args {
		args = append(args, psQuote(arg))
	}
	delayMs := int(time.Until(req.RestartAt) / time.Millisecond)
	if delayMs < 0 {
		delayMs = 0
	}
	replace := ""
	if req.ReplaceExecutable {
		replace = fmt.Sprintf(`  Remove-Item -LiteralPath %s -Force -ErrorAction SilentlyContinue
  Move-Item -LiteralPath %s -Destination %s -Force
`, psQuote(req.CurrentExe), psQuote(req.NextExe), psQuote(req.CurrentExe))
	}
	return fmt.Sprintf(`$ErrorActionPreference = 'Stop'
Start-Sleep -Milliseconds %d
try {
  Wait-Process -Id %d -Timeout 30 -ErrorAction SilentlyContinue
%s
  Start-Process -FilePath %s -ArgumentList @(%s) -WorkingDirectory %s -WindowStyle Hidden
  "restarted at $(Get-Date -Format o)" | Out-File -FilePath %s -Append -Encoding utf8
} catch {
  "restart failed at $(Get-Date -Format o): $($_.Exception.Message)" | Out-File -FilePath %s -Append -Encoding utf8
  exit 1
} finally {
  Remove-Item -LiteralPath %s -Force -ErrorAction SilentlyContinue
}
`,
		delayMs,
		req.PID,
		replace,
		psQuote(req.CurrentExe),
		strings.Join(args, ", "),
		psQuote(req.WorkDir),
		psQuote(logPath),
		psQuote(logPath),
		psQuote(helperPath),
	)
}

func linuxRestartScript(req updateRestartRequest, logPath, helperPath string) string {
	args := make([]string, 0, len(req.Args))
	for _, arg := range req.Args {
		args = append(args, shQuote(arg))
	}
	delay := time.Until(req.RestartAt).Seconds()
	if delay < 0 {
		delay = 0
	}
	replace := ""
	if req.ReplaceExecutable {
		replace = fmt.Sprintf("mv -f %s %s\n", shQuote(req.NextExe), shQuote(req.CurrentExe))
	}
	return fmt.Sprintf(`#!/bin/sh
set -eu
sleep %.3f
i=0
while kill -0 %d 2>/dev/null && [ "$i" -lt 300 ]; do
  i=$((i + 1))
  sleep 0.1
done
%s
cd %s
nohup %s %s >> %s 2>&1 &
printf 'restarted at %%s\n' "$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)" >> %s
rm -f %s
`,
		delay,
		req.PID,
		replace,
		shQuote(req.WorkDir),
		shQuote(req.CurrentExe),
		strings.Join(args, " "),
		shQuote(logPath),
		shQuote(logPath),
		shQuote(helperPath),
	)
}

func runGitInDir(ctx context.Context, dir, proxyURL string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Args = append([]string{"git", "-c", "core.hooksPath=" + filepath.Join(os.TempDir(), "gpufleet-disabled-hooks")}, args...)
	cmd.Dir = dir
	cmd.Env = updateProxyEnv(append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=Never"), proxyURL)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	text := limitText(stdout.String(), updateOutputLimit)
	if ctx.Err() != nil {
		return text, ctx.Err()
	}
	if err != nil {
		combined := limitText(stdout.String()+stderr.String(), updateOutputLimit)
		return combined, fmt.Errorf("%s: %s", strings.Join(args, " "), strings.TrimSpace(combined))
	}
	return text, nil
}

func updateProxyEnv(env []string, proxyURL string) []string {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return env
	}
	env = append(env,
		"HTTP_PROXY="+proxyURL,
		"HTTPS_PROXY="+proxyURL,
		"ALL_PROXY="+proxyURL,
		"http_proxy="+proxyURL,
		"https_proxy="+proxyURL,
		"all_proxy="+proxyURL,
	)
	return env
}

func currentExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", fmt.Errorf("resolve current executable path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, nil
	}
	return resolved, nil
}

func writableDir(dir string) error {
	probe, err := os.CreateTemp(dir, ".gpufleet-update-write-*")
	if err != nil {
		return err
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}

func fileExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return nil
}

func lookPath(name string) error {
	_, err := exec.LookPath(name)
	return err
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Dir(os.Args[0])
	}
	return wd
}

func psQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func shQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func parseAheadBehind(raw string) (int, int) {
	parts := strings.Fields(raw)
	if len(parts) < 2 {
		return 0, 0
	}
	ahead, _ := strconv.Atoi(parts[0])
	behind, _ := strconv.Atoi(parts[1])
	return ahead, behind
}

func updateMessage(status updateStatus) string {
	switch {
	case status.Dirty:
		return "server working tree has uncommitted changes"
	case status.Ahead > 0 && status.Behind > 0:
		return "local and upstream branches have diverged"
	case status.Ahead > 0:
		return "local branch is ahead of upstream"
	case status.Available:
		return "update available"
	default:
		return "already up to date"
	}
}

func cleanCommandError(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

func limitText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

func shortCommit(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func sanitizeGitRemote(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.User = nil
		return parsed.String()
	}
	return raw
}
