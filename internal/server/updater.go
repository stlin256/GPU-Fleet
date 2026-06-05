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
	"strconv"
	"strings"
	"time"
)

const (
	updateCheckTimeout = 25 * time.Second
	updatePullTimeout  = 60 * time.Second
	updateOutputLimit  = 6000
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

type updateApplyResponse struct {
	OK              bool         `json:"ok"`
	Status          updateStatus `json:"status"`
	Output          string       `json:"output,omitempty"`
	RestartRequired bool         `json:"restart_required"`
}

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

	before := status.LocalCommit
	pullCtx, pullCancel := context.WithTimeout(r.Context(), updatePullTimeout)
	output, err := a.runGit(pullCtx, "pull", "--ff-only")
	pullCancel()
	if err != nil {
		_ = a.meta.AddAudit("server_update_failed", limitText(err.Error(), 300))
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("git pull failed: %s", limitText(err.Error(), 500)))
		return
	}

	finalCtx, finalCancel := context.WithTimeout(r.Context(), updateCheckTimeout)
	finalStatus := a.gitUpdateStatus(finalCtx)
	finalCancel()
	restartRequired := before != "" && finalStatus.LocalCommit != "" && before != finalStatus.LocalCommit
	_ = a.meta.AddAudit("server_update_pulled", fmt.Sprintf("pulled fast-forward update from %s to %s", shortCommit(before), shortCommit(finalStatus.LocalCommit)))
	writeJSON(w, http.StatusOK, updateApplyResponse{
		OK:              true,
		Status:          finalStatus,
		Output:          limitText(output, updateOutputLimit),
		RestartRequired: restartRequired,
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
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Args = append([]string{"git", "-c", "core.hooksPath=" + filepath.Join(os.TempDir(), "gpufleet-disabled-hooks")}, args...)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=Never")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
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
