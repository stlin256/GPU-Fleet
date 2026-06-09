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

	"gpufleet/internal/version"
)

const (
	updateCheckTimeout         = 25 * time.Second
	updatePullTimeout          = 60 * time.Second
	updateBuildTimeout         = 3 * time.Minute
	updateRestartDelay         = 1200 * time.Millisecond
	autoUpdateInterval         = 30 * time.Minute
	updateCheckInterval        = time.Hour
	updateOutputLimit          = 6000
	updateSourceEntry          = "./cmd/gpufleet-server"
	updateRestartLogName       = "gpufleet-update-restart.log"
	updateRestartGrace         = 10 * time.Minute
	updateRecoveryCheckTimeout = 2 * time.Second
	minCommitPrefixLen         = 7
	trustedUpdateHost          = "github.com"
	trustedUpdateRepo          = "stlin256/gpu-fleet"
)

type updateStatus struct {
	Available      bool                    `json:"available"`
	Supported      bool                    `json:"supported"`
	Dirty          bool                    `json:"dirty"`
	Branch         string                  `json:"branch,omitempty"`
	Remote         string                  `json:"remote,omitempty"`
	Upstream       string                  `json:"upstream,omitempty"`
	LocalCommit    string                  `json:"local_commit,omitempty"`
	RemoteCommit   string                  `json:"remote_commit,omitempty"`
	RunningVersion string                  `json:"running_version,omitempty"`
	RunningCommit  string                  `json:"running_commit,omitempty"`
	RunningBuild   string                  `json:"running_build_time,omitempty"`
	RepoVersion    string                  `json:"repo_version,omitempty"`
	BinaryOutdated bool                    `json:"binary_outdated"`
	Behind         int                     `json:"behind"`
	Ahead          int                     `json:"ahead"`
	CheckedAt      time.Time               `json:"checked_at"`
	SupplyChain    updateSupplyChainStatus `json:"supply_chain"`
	Failed         bool                    `json:"failed,omitempty"`
	Message        string                  `json:"message,omitempty"`
	Detail         string                  `json:"detail,omitempty"`
}

type updateSupplyChainStatus struct {
	OK                bool     `json:"ok"`
	Blocked           bool     `json:"blocked"`
	RemoteTrusted     bool     `json:"remote_trusted"`
	RemoteKind        string   `json:"remote_kind,omitempty"`
	RemoteHost        string   `json:"remote_host,omitempty"`
	RemoteRepository  string   `json:"remote_repository,omitempty"`
	UpstreamBound     bool     `json:"upstream_bound"`
	FastForwardOnly   bool     `json:"fast_forward_only"`
	WorktreeClean     bool     `json:"worktree_clean"`
	ExactTargetCommit bool     `json:"exact_target_commit"`
	Warnings          []string `json:"warnings,omitempty"`
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
	Notice           *UpdateNotice          `json:"notice,omitempty"`
	TargetCommit     string                 `json:"target_commit,omitempty"`
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
	BackupExe         string
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

	if r.URL.Query().Get("cached") == "1" && r.URL.Query().Get("fresh") != "1" {
		if cached, ok := a.cachedUpdateStatusLocked(updateMonitorInterval(a.meta.ServiceConfig().AutoUpdateOn())); ok {
			writeJSON(w, http.StatusOK, cached)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), updateCheckTimeout)
	defer cancel()
	writeJSON(w, http.StatusOK, a.checkUpdateStatusLocked(ctx))
}

func (a *App) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	a.updateMu.Lock()
	defer a.updateMu.Unlock()

	_ = a.addAuditForRequest(r, "server_update_requested", "manual server update requested from settings", "")
	response, statusCode, message := a.applyUpdateLocked(r.Context(), false)
	if statusCode >= http.StatusBadRequest {
		writeError(w, statusCode, message)
		return
	}
	writeJSON(w, statusCode, response)
}

func (a *App) handleUpdateNotice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notice": a.meta.TakeUpdateNotice()})
}

func (a *App) startUpdateMonitorLoop() {
	go func() {
		for {
			if a.runUpdateMonitorCycle() {
				return
			}
			timer := time.NewTimer(updateMonitorInterval(a.meta.ServiceConfig().AutoUpdateOn()))
			select {
			case <-timer.C:
			case <-a.updateMonitorWake:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			}
		}
	}()
}

func (a *App) applyUpdateLocked(ctx context.Context, automatic bool) (updateApplyResponse, int, string) {
	startedAt := time.Now().UTC()
	checkCtx, checkCancel := context.WithTimeout(ctx, updateCheckTimeout)
	status := a.checkUpdateStatusLocked(checkCtx)
	checkCancel()
	return a.applyUpdateLockedWithStatus(ctx, automatic, status, startedAt)
}

func (a *App) applyUpdateLockedWithStatus(ctx context.Context, automatic bool, status updateStatus, startedAt time.Time) (updateApplyResponse, int, string) {
	if !status.Supported {
		return updateApplyResponse{}, http.StatusBadRequest, status.Message
	}
	if status.Upstream == "" {
		message := "current branch has no upstream"
		_ = a.meta.AddAudit("server_update_blocked", message)
		return updateApplyResponse{}, http.StatusBadRequest, message
	}
	if status.Dirty {
		message := "server working tree has uncommitted changes"
		_ = a.meta.AddAudit("server_update_blocked", message)
		return updateApplyResponse{}, http.StatusConflict, message
	}
	if status.Ahead > 0 {
		message := "local branch is ahead of upstream; fast-forward update is not available"
		_ = a.meta.AddAudit("server_update_blocked", message)
		return updateApplyResponse{}, http.StatusConflict, message
	}
	if status.Failed {
		_ = a.meta.AddAudit("server_update_failed", status.Message)
		return updateApplyResponse{}, http.StatusBadGateway, status.Message
	}
	if status.SupplyChain.Blocked {
		message := updateSupplyChainBlockMessage(status.SupplyChain)
		_ = a.meta.AddAudit("server_update_blocked", message)
		return updateApplyResponse{}, http.StatusPreconditionFailed, message
	}
	if (status.Available || status.BinaryOutdated) && !status.SupplyChain.ExactTargetCommit {
		message := "update target commit could not be verified as an exact commit hash"
		_ = a.meta.AddAudit("server_update_blocked", message)
		return updateApplyResponse{}, http.StatusPreconditionFailed, message
	}
	if !status.Available && !status.BinaryOutdated {
		return updateApplyResponse{OK: true, Status: status, RestartRequired: false}, http.StatusOK, ""
	}

	deps := a.checkUpdateDependencies()
	if !deps.OK {
		_ = a.meta.AddAudit("server_update_failed", "missing update dependencies: "+strings.Join(deps.Missing, ", "))
		return updateApplyResponse{}, http.StatusPreconditionFailed, "missing update dependencies: " + strings.Join(deps.Missing, ", ")
	}

	exePath, err := currentExecutablePath()
	if err != nil {
		_ = a.meta.AddAudit("server_update_failed", limitText(err.Error(), 300))
		return updateApplyResponse{}, http.StatusPreconditionFailed, err.Error()
	}
	nextPath := exePath + ".next"
	backupPath := exePath + ".bak"
	_ = os.Remove(nextPath)

	buildCtx, buildCancel := context.WithTimeout(ctx, updateBuildTimeout)
	targetCommit := status.RemoteCommit
	if !status.Available && status.BinaryOutdated {
		targetCommit = status.LocalCommit
	}
	noticeBeforeStatus := updateNoticeBeforeStatus(status, targetCommit)
	beforeChangelog, _ := a.changelogAt(ctx, noticeBeforeStatus.LocalCommit)
	targetChangelog, _ := a.changelogAt(ctx, targetCommit)
	buildResult, err := a.updateBuildServer(buildCtx, updateBuildRequest{
		RepoDir:      a.config.RepoDir,
		RemoteCommit: targetCommit,
		OutputPath:   nextPath,
		ProxyURL:     a.meta.ServiceConfig().UpdateProxy,
	})
	buildCancel()
	if err != nil {
		_ = os.Remove(nextPath)
		_ = a.meta.AddAudit("server_update_failed", limitText(err.Error(), 300))
		return updateApplyResponse{}, http.StatusInternalServerError, fmt.Sprintf("build update failed: %s", limitText(err.Error(), 500))
	}

	before := status.LocalCommit
	output := ""
	if status.Available {
		pullCtx, pullCancel := context.WithTimeout(ctx, updatePullTimeout)
		output, err = a.runGit(pullCtx, "pull", "--ff-only")
		pullCancel()
		if err != nil {
			_ = os.Remove(buildResult.OutputPath)
			_ = a.meta.AddAudit("server_update_failed", limitText(err.Error(), 300))
			return updateApplyResponse{}, http.StatusInternalServerError, fmt.Sprintf("git pull failed: %s", limitText(err.Error(), 500))
		}
	}

	finalCtx, finalCancel := context.WithTimeout(ctx, updateCheckTimeout)
	finalStatus := a.checkUpdateStatusLocked(finalCtx)
	finalCancel()
	restartRequired := status.BinaryOutdated || (before != "" && finalStatus.LocalCommit != "" && before != finalStatus.LocalCommit)
	if restartRequired {
		restartAt := time.Now().UTC().Add(updateRestartDelay)
		restartReq := updateRestartRequest{
			CurrentExe:        exePath,
			NextExe:           buildResult.OutputPath,
			BackupExe:         backupPath,
			Args:              append([]string(nil), os.Args[1:]...),
			WorkDir:           mustGetwd(),
			PID:               os.Getpid(),
			RestartAt:         restartAt,
			ReplaceExecutable: true,
		}
		if err := a.updateScheduleRestart(restartReq); err != nil {
			_ = os.Remove(buildResult.OutputPath)
			_ = a.meta.AddAudit("server_update_failed", limitText(err.Error(), 300))
			return updateApplyResponse{}, http.StatusInternalServerError, fmt.Sprintf("schedule restart failed: %s", limitText(err.Error(), 500))
		}
		noticeKind := "update"
		if automatic {
			noticeKind = "auto_update"
		}
		notice, _ := a.saveUpdateNoticeFromChangelog(noticeKind, noticeBeforeStatus, finalStatus, targetCommit, beforeChangelog, targetChangelog, startedAt, restartAt)
		if status.BinaryOutdated && !status.Available {
			_ = a.meta.AddAudit("server_update_scheduled", fmt.Sprintf("rebuilt %s from repository commit %s and scheduled automatic restart", version.String(), shortCommit(targetCommit)))
		} else {
			_ = a.meta.AddAudit("server_update_scheduled", fmt.Sprintf("pulled %s -> %s and scheduled automatic restart", shortCommit(before), shortCommit(finalStatus.LocalCommit)))
		}
		go func() {
			time.Sleep(updateRestartDelay)
			if a.updateExit != nil {
				a.updateExit()
				return
			}
			os.Exit(0)
		}()
		return updateApplyResponse{
			OK:               true,
			Status:           finalStatus,
			Output:           limitText(output, updateOutputLimit),
			BuildOutput:      limitText(buildResult.Output, updateOutputLimit),
			DependencyStatus: deps,
			Notice:           notice,
			TargetCommit:     targetCommit,
			RestartRequired:  true,
			Restarting:       true,
			RestartAt:        restartAt,
		}, http.StatusOK, ""
	}

	_ = os.Remove(buildResult.OutputPath)
	return updateApplyResponse{
		OK:               true,
		Status:           finalStatus,
		Output:           limitText(output, updateOutputLimit),
		BuildOutput:      limitText(buildResult.Output, updateOutputLimit),
		DependencyStatus: deps,
		TargetCommit:     targetCommit,
		RestartRequired:  false,
		Restarting:       false,
	}, http.StatusOK, ""
}

func (a *App) runUpdateMonitorCycle() bool {
	if !a.meta.SetupComplete() {
		return false
	}
	automatic := a.meta.ServiceConfig().AutoUpdateOn()
	a.updateMu.Lock()
	defer a.updateMu.Unlock()

	checkCtx, checkCancel := context.WithTimeout(context.Background(), updateCheckTimeout)
	status := a.checkUpdateStatusLocked(checkCtx)
	checkCancel()
	if !automatic || !status.Supported || status.Failed || status.Upstream == "" || status.Dirty || status.Ahead > 0 || status.SupplyChain.Blocked || (!status.Available && !status.BinaryOutdated) {
		return false
	}
	if automaticUpdateRecentlyCompletedForTarget(a.meta.PeekUpdateNotice(), status, time.Now().UTC()) {
		if a.logger != nil {
			a.logger.Printf("auto update skipped: recent update already targeted %s", shortCommit(status.LocalCommit))
		}
		return false
	}
	response, statusCode, message := a.applyUpdateLockedWithStatus(context.Background(), true, status, time.Now().UTC())
	if statusCode >= http.StatusBadRequest {
		if a.logger != nil {
			a.logger.Printf("auto update skipped: %s", message)
		}
		return false
	}
	return response.Restarting
}

func (a *App) schedulePendingExecutableRecovery() bool {
	exePath, err := currentExecutablePath()
	if err != nil {
		return false
	}
	nextPath := exePath + ".next"
	if info, err := os.Stat(nextPath); err != nil || info.IsDir() || info.Size() == 0 {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), updateRecoveryCheckTimeout)
	defer cancel()
	localCommit, err := a.runGit(ctx, "rev-parse", "HEAD")
	if err != nil {
		return false
	}
	localCommit = strings.TrimSpace(localCommit)
	repoVersion, _ := a.repoVersionAt(ctx, "HEAD")
	if !binaryOutdated(updateStatus{
		LocalCommit:    localCommit,
		RunningCommit:  version.Commit,
		RepoVersion:    repoVersion,
		RunningVersion: version.Version,
	}) {
		return false
	}
	restartAt := time.Now().UTC().Add(updateRestartDelay)
	req := updateRestartRequest{
		CurrentExe:        exePath,
		NextExe:           nextPath,
		BackupExe:         exePath + ".bak",
		Args:              append([]string(nil), os.Args[1:]...),
		WorkDir:           mustGetwd(),
		PID:               os.Getpid(),
		RestartAt:         restartAt,
		ReplaceExecutable: true,
	}
	if err := a.updateScheduleRestart(req); err != nil {
		if a.logger != nil {
			a.logger.Printf("pending update recovery failed: %v", err)
		}
		return false
	}
	if a.logger != nil {
		a.logger.Printf("pending update recovery scheduled for repository commit %s", shortCommit(localCommit))
	}
	go func() {
		time.Sleep(updateRestartDelay)
		if a.updateExit != nil {
			a.updateExit()
			return
		}
		os.Exit(0)
	}()
	return true
}

func updateMonitorInterval(autoUpdateOn bool) time.Duration {
	if autoUpdateOn {
		return autoUpdateInterval
	}
	return updateCheckInterval
}

func (a *App) wakeUpdateMonitor() {
	if a.updateMonitorWake == nil {
		return
	}
	select {
	case a.updateMonitorWake <- struct{}{}:
	default:
	}
}

func (a *App) checkUpdateStatusLocked(ctx context.Context) updateStatus {
	status := a.gitUpdateStatus(ctx)
	a.updateStatusCache = status
	a.updateStatusCacheOK = true
	return status
}

func (a *App) cachedUpdateStatusLocked(maxAge time.Duration) (updateStatus, bool) {
	if !a.updateStatusCacheOK || a.updateStatusCache.CheckedAt.IsZero() {
		return updateStatus{}, false
	}
	if maxAge <= 0 || time.Since(a.updateStatusCache.CheckedAt) > maxAge {
		return updateStatus{}, false
	}
	return a.updateStatusCache, true
}

func (a *App) saveAutomaticUpdateNotice(ctx context.Context, beforeStatus, finalStatus updateStatus, targetCommit string, startedAt, restartAt time.Time) error {
	summary, summaryEN := a.updateSummary(ctx, beforeStatus.LocalCommit, targetCommit)
	_, err := a.saveUpdateNoticeWithSummary("auto_update", beforeStatus, finalStatus, targetCommit, summary, summaryEN, startedAt, restartAt)
	return err
}

func (a *App) saveUpdateNoticeFromChangelog(kind string, beforeStatus, finalStatus updateStatus, targetCommit, beforeRaw, targetRaw string, startedAt, restartAt time.Time) (*UpdateNotice, error) {
	summary, summaryEN := updateSummaryFromChangelog(beforeRaw, targetRaw)
	return a.saveUpdateNoticeWithSummary(kind, beforeStatus, finalStatus, targetCommit, summary, summaryEN, startedAt, restartAt)
}

func (a *App) saveUpdateNoticeWithSummary(kind string, beforeStatus, finalStatus updateStatus, targetCommit string, summary, summaryEN []string, startedAt, restartAt time.Time) (*UpdateNotice, error) {
	if strings.TrimSpace(kind) == "" {
		kind = "update"
	}
	if len(summary) == 0 && len(summaryEN) == 0 {
		summary = []string{"无更新说明"}
		summaryEN = []string{"No update notes."}
	}
	currentVersion := finalStatus.RepoVersion
	if currentVersion == "" {
		currentVersion = beforeStatus.RepoVersion
	}
	notice := UpdateNotice{
		ID:              strings.TrimSpace(targetCommit) + "-" + strconv.FormatInt(time.Now().UTC().Unix(), 10),
		Kind:            kind,
		Product:         version.Product,
		PreviousCommit:  beforeStatus.LocalCommit,
		TargetCommit:    targetCommit,
		CurrentCommit:   finalStatus.LocalCommit,
		PreviousVersion: beforeStatus.RepoVersion,
		CurrentVersion:  currentVersion,
		StartedAt:       startedAt,
		CompletedAt:     restartAt,
		UpdatedAt:       time.Now().UTC(),
		Summary:         summary,
		SummaryEN:       summaryEN,
	}
	if err := a.meta.SaveUpdateNotice(notice); err != nil {
		return nil, err
	}
	return &notice, nil
}

func updateNoticeBeforeStatus(status updateStatus, targetCommit string) updateStatus {
	before := status
	runningCommit := strings.TrimSpace(status.RunningCommit)
	targetCommit = strings.TrimSpace(targetCommit)
	if runningCommit != "" && runningCommit != "dev" && runningCommit != targetCommit {
		before.LocalCommit = runningCommit
		if strings.TrimSpace(status.RunningVersion) != "" {
			before.RepoVersion = status.RunningVersion
		}
	}
	return before
}

func (a *App) updateSummary(ctx context.Context, beforeCommit, targetCommit string) ([]string, []string) {
	if strings.TrimSpace(targetCommit) == "" {
		targetCommit = "HEAD"
	}
	afterRaw, err := a.changelogAt(ctx, targetCommit)
	if err != nil || strings.TrimSpace(afterRaw) == "" {
		return []string{"无更新说明"}, []string{"No update notes."}
	}
	beforeRaw := ""
	if strings.TrimSpace(beforeCommit) != "" {
		beforeRaw, _ = a.changelogAt(ctx, beforeCommit)
	}
	if strings.TrimSpace(beforeRaw) == strings.TrimSpace(afterRaw) {
		return []string{"无更新说明"}, []string{"No update notes."}
	}
	return updateSummaryFromChangelog(beforeRaw, afterRaw)
}

func updateSummaryFromChangelog(beforeRaw, afterRaw string) ([]string, []string) {
	if strings.TrimSpace(afterRaw) == "" || strings.TrimSpace(beforeRaw) == strings.TrimSpace(afterRaw) {
		return []string{"无更新说明"}, []string{"No update notes."}
	}
	afterEntries := version.ChangelogFromMarkdown(afterRaw)
	if len(afterEntries) == 0 {
		return []string{"无更新说明"}, []string{"No update notes."}
	}
	beforeEntries := version.ChangelogFromMarkdown(beforeRaw)
	afterTop := afterEntries[0]
	if len(beforeEntries) == 0 || beforeEntries[0].Version != afterTop.Version {
		zh := changelogEntryItems(afterTop, false)
		en := changelogEntryItems(afterTop, true)
		if len(zh) == 0 && afterTop.Title != "" {
			zh = append(zh, afterTop.Title)
		}
		if len(en) == 0 && afterTop.TitleEN != "" {
			en = append(en, afterTop.TitleEN)
		}
		return updateSummaryFallback(zh, en)
	}
	zh := newChangelogItems(changelogEntryItems(afterTop, false), changelogEntryItems(beforeEntries[0], false))
	en := newChangelogItems(changelogEntryItems(afterTop, true), changelogEntryItems(beforeEntries[0], true))
	return updateSummaryFallback(zh, en)
}

func (a *App) changelogAt(ctx context.Context, rev string) (string, error) {
	showCtx, cancel := context.WithTimeout(ctx, updateCheckTimeout)
	defer cancel()
	return a.runGit(showCtx, "show", rev+":CHANGELOG.md")
}

func changelogEntryItems(entry version.ChangelogEntry, english bool) []string {
	var items []string
	add := func(values []string) {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value != "" {
				items = append(items, value)
			}
		}
	}
	if english {
		add(entry.AddedEN)
		add(entry.ChangedEN)
		add(entry.SecurityEN)
		add(entry.FixedEN)
		return items
	}
	add(entry.Added)
	add(entry.Changed)
	add(entry.Security)
	add(entry.Fixed)
	return items
}

func newChangelogItems(after, before []string) []string {
	seen := map[string]bool{}
	for _, item := range before {
		seen[normalizeChangelogItem(item)] = true
	}
	var out []string
	for _, item := range after {
		if !seen[normalizeChangelogItem(item)] {
			out = append(out, item)
		}
	}
	return out
}

func normalizeChangelogItem(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func updateSummaryFallback(zh, en []string) ([]string, []string) {
	if len(zh) == 0 {
		zh = []string{"无更新说明"}
	}
	if len(en) == 0 {
		en = []string{"No update notes."}
	}
	return zh, en
}

func (a *App) gitUpdateStatus(ctx context.Context) updateStatus {
	status := updateStatus{
		CheckedAt:      time.Now().UTC(),
		Message:        "checking repository",
		RunningVersion: version.Version,
		RunningCommit:  version.Commit,
		RunningBuild:   version.BuildTime,
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
	remoteRaw := ""
	if remote, err := a.runGit(ctx, "remote", "get-url", "origin"); err == nil {
		remoteRaw = strings.TrimSpace(remote)
		status.Remote = sanitizeGitRemote(remoteRaw)
	}
	if local, err := a.runGit(ctx, "rev-parse", "HEAD"); err == nil {
		status.LocalCommit = strings.TrimSpace(local)
	}
	if repoVersion, err := a.repoVersionAt(ctx, "HEAD"); err == nil {
		status.RepoVersion = repoVersion
	}
	status.Dirty = a.workingTreeDirty(ctx)

	upstream, err := a.runGit(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err != nil {
		status.Message = "current branch has no upstream"
		status.SupplyChain = updateSupplyChainStatusForRemote(status, remoteRaw)
		return status
	}
	status.Upstream = strings.TrimSpace(upstream)
	if upstreamRemote := remoteNameFromUpstream(status.Upstream); upstreamRemote != "" {
		if remote, err := a.runGit(ctx, "remote", "get-url", upstreamRemote); err == nil {
			remoteRaw = strings.TrimSpace(remote)
			status.Remote = sanitizeGitRemote(remoteRaw)
		}
	}

	if _, err := a.runGit(ctx, "fetch", "--quiet", "--prune"); err != nil {
		detail := limitText(cleanCommandError(err), updateOutputLimit)
		status.Failed = true
		status.Message = gitFailureMessage("fetch", detail, a.meta.ServiceConfig().UpdateProxy)
		status.Detail = detail
		status.SupplyChain = updateSupplyChainStatusForRemote(status, remoteRaw)
		return status
	}
	if remoteCommit, err := a.runGit(ctx, "rev-parse", "@{u}"); err == nil {
		status.RemoteCommit = strings.TrimSpace(remoteCommit)
	}
	if counts, err := a.runGit(ctx, "rev-list", "--left-right", "--count", "HEAD...@{u}"); err == nil {
		status.Ahead, status.Behind = parseAheadBehind(counts)
	}
	status.Available = status.Behind > 0
	status.BinaryOutdated = binaryOutdated(status)
	status.SupplyChain = updateSupplyChainStatusForRemote(status, remoteRaw)
	status.Message = updateMessage(status)
	return status
}

func (a *App) repoVersionAt(ctx context.Context, rev string) (string, error) {
	raw, err := a.runGit(ctx, "show", rev+":internal/version/version.go")
	if err != nil {
		return "", err
	}
	return parseVersionGo(raw), nil
}

func parseVersionGo(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Version") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		return strings.Trim(strings.TrimSpace(parts[1]), `"`)
	}
	return ""
}

func binaryOutdated(status updateStatus) bool {
	if status.LocalCommit == "" {
		return false
	}
	if status.RunningCommit != "" && status.RunningCommit != "dev" && !commitRefMatches(status.RunningCommit, status.LocalCommit) {
		return true
	}
	if status.RepoVersion != "" && status.RunningVersion != "" && status.RunningVersion != status.RepoVersion {
		return true
	}
	return false
}

func commitRefMatches(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	if strings.EqualFold(left, right) {
		return true
	}
	if len(left) >= minCommitPrefixLen && len(right) >= minCommitPrefixLen {
		return strings.HasPrefix(strings.ToLower(left), strings.ToLower(right)) || strings.HasPrefix(strings.ToLower(right), strings.ToLower(left))
	}
	return false
}

func automaticUpdateRecentlyCompletedForTarget(notice *UpdateNotice, status updateStatus, now time.Time) bool {
	if notice == nil || status.Available || !status.BinaryOutdated || strings.TrimSpace(status.LocalCommit) == "" {
		return false
	}
	if !commitRefMatches(notice.TargetCommit, status.LocalCommit) && !commitRefMatches(notice.CurrentCommit, status.LocalCommit) {
		return false
	}
	completedAt := notice.CompletedAt
	if completedAt.IsZero() {
		completedAt = notice.UpdatedAt
	}
	if completedAt.IsZero() {
		return false
	}
	return completedAt.Add(updateRestartGrace).After(now)
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

	buildTime := time.Now().UTC().Format(time.RFC3339)
	ldflags := fmt.Sprintf("-X gpufleet/internal/version.Commit=%s -X gpufleet/internal/version.BuildTime=%s", req.RemoteCommit, buildTime)
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags", ldflags, "-o", req.OutputPath, updateSourceEntry)
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
		if req.BackupExe == "" {
			req.BackupExe = req.CurrentExe + ".bak"
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
	if req.BackupExe == "" {
		req.BackupExe = req.CurrentExe + ".bak"
	}
	args := make([]string, 0, len(req.Args))
	for _, arg := range req.Args {
		args = append(args, psQuote(arg))
	}
	delayMs := int(time.Until(req.RestartAt) / time.Millisecond)
	if delayMs < 0 {
		delayMs = 0
	}
	replace := ""
	rollback := ""
	if req.ReplaceExecutable {
		replace = fmt.Sprintf(`  $waitUntil = (Get-Date).AddSeconds(60)
  while ((Get-Date) -lt $waitUntil) {
    $existing = Get-Process -Id %d -ErrorAction SilentlyContinue
    if (-not $existing) { break }
    Start-Sleep -Milliseconds 250
  }
  if (Get-Process -Id %d -ErrorAction SilentlyContinue) {
    throw "current process did not exit before replacement"
  }
  $replaceUntil = (Get-Date).AddSeconds(60)
  while ($true) {
    try {
      Remove-Item -LiteralPath %s -Force -ErrorAction SilentlyContinue
      Copy-Item -LiteralPath %s -Destination %s -Force
      Move-Item -LiteralPath %s -Destination %s -Force
      break
    } catch {
      if ((Get-Date) -ge $replaceUntil) { throw }
      Start-Sleep -Milliseconds 250
    }
  }
`, req.PID, req.PID, psQuote(req.BackupExe), psQuote(req.CurrentExe), psQuote(req.BackupExe), psQuote(req.NextExe), psQuote(req.CurrentExe))
		rollback = fmt.Sprintf(`  if (Test-Path -LiteralPath %s) {
    Copy-Item -LiteralPath %s -Destination %s -Force
  }
`, psQuote(req.BackupExe), psQuote(req.BackupExe), psQuote(req.CurrentExe))
	}
	return fmt.Sprintf(`$ErrorActionPreference = 'Stop'
Start-Sleep -Milliseconds %d
try {
%s
  Start-Process -FilePath %s -ArgumentList @(%s) -WorkingDirectory %s -WindowStyle Hidden
  "restarted at $(Get-Date -Format o)" | Out-File -FilePath %s -Append -Encoding utf8
} catch {
%s
  "restart failed at $(Get-Date -Format o): $($_.Exception.Message)" | Out-File -FilePath %s -Append -Encoding utf8
  exit 1
} finally {
  Remove-Item -LiteralPath %s -Force -ErrorAction SilentlyContinue
}
`,
		delayMs,
		replace,
		psQuote(req.CurrentExe),
		strings.Join(args, ", "),
		psQuote(req.WorkDir),
		psQuote(logPath),
		rollback,
		psQuote(logPath),
		psQuote(helperPath),
	)
}

func linuxRestartScript(req updateRestartRequest, logPath, helperPath string) string {
	if req.BackupExe == "" {
		req.BackupExe = req.CurrentExe + ".bak"
	}
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
		replace = fmt.Sprintf(`i=0
while kill -0 %d 2>/dev/null && [ "$i" -lt 600 ]; do
  i=$((i + 1))
  sleep 0.1
done
if kill -0 %d 2>/dev/null; then
  printf 'current process did not exit before replacement at %%s\n' "$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)" >> %s
  exit 1
fi
i=0
while [ "$i" -lt 600 ]; do
  rm -f %s
  if cp -p %s %s && mv -f %s %s && chmod 0755 %s; then
    break
  fi
  i=$((i + 1))
  sleep 0.1
done
if [ "$i" -ge 600 ]; then
  cp -p %s %s 2>/dev/null || true
  printf 'replacement failed and rollback was attempted at %%s\n' "$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)" >> %s
  exit 1
fi
`, req.PID, req.PID, shQuote(logPath), shQuote(req.BackupExe), shQuote(req.CurrentExe), shQuote(req.BackupExe), shQuote(req.NextExe), shQuote(req.CurrentExe), shQuote(req.CurrentExe), shQuote(req.BackupExe), shQuote(req.CurrentExe), shQuote(logPath))
	}
	return fmt.Sprintf(`#!/bin/sh
set -eu
sleep %.3f
%s
already_running=0
for exe in /proc/[0-9]*/exe; do
  target=$(readlink "$exe" 2>/dev/null || true)
  if [ "$target" = %s ]; then
    already_running=1
    break
  fi
done
if [ "$already_running" -eq 0 ]; then
  cd %s
  nohup %s %s >> %s 2>&1 &
else
  printf 'restart process already running at %%s\n' "$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)" >> %s
fi
printf 'restarted at %%s\n' "$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)" >> %s
rm -f %s
`,
		delay,
		replace,
		shQuote(req.CurrentExe),
		shQuote(req.WorkDir),
		shQuote(req.CurrentExe),
		strings.Join(args, " "),
		shQuote(logPath),
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
	case status.Failed:
		return status.Message
	case status.SupplyChain.Blocked:
		return updateSupplyChainBlockMessage(status.SupplyChain)
	case status.Dirty:
		return "server working tree has uncommitted changes"
	case status.Ahead > 0 && status.Behind > 0:
		return "local and upstream branches have diverged"
	case status.Ahead > 0:
		return "local branch is ahead of upstream"
	case status.Available:
		return "update available"
	case status.BinaryOutdated:
		return "running server binary was not built from the current repository checkout"
	default:
		return "already up to date"
	}
}

func updateSupplyChainBlockMessage(status updateSupplyChainStatus) string {
	if len(status.Warnings) > 0 {
		return "update supply-chain check blocked the operation: " + strings.Join(status.Warnings, "; ")
	}
	return "update supply-chain check blocked the operation"
}

func updateSupplyChainStatusForRemote(status updateStatus, remoteRaw string) updateSupplyChainStatus {
	kind, host, repo, trusted := classifyGitRemote(remoteRaw)
	out := updateSupplyChainStatus{
		RemoteTrusted:    trusted || kind == "local",
		RemoteKind:       kind,
		RemoteHost:       host,
		RemoteRepository: repo,
		UpstreamBound:    strings.TrimSpace(status.Upstream) != "",
		FastForwardOnly:  status.Ahead == 0,
		WorktreeClean:    !status.Dirty,
	}
	targetCommit := status.RemoteCommit
	if !status.Available && status.BinaryOutdated {
		targetCommit = status.LocalCommit
	}
	if targetCommit == "" {
		targetCommit = status.LocalCommit
	}
	out.ExactTargetCommit = isFullCommitHash(targetCommit)
	if kind == "network" && !trusted {
		out.Blocked = true
		if host == "" || repo == "" {
			out.Warnings = append(out.Warnings, "network remote could not be identified")
		} else {
			out.Warnings = append(out.Warnings, fmt.Sprintf("network remote %s/%s is not %s/%s", host, repo, trustedUpdateHost, trustedUpdateRepo))
		}
	}
	if !out.UpstreamBound {
		out.Warnings = append(out.Warnings, "current branch has no upstream")
	}
	if !out.WorktreeClean {
		out.Warnings = append(out.Warnings, "server working tree has uncommitted changes")
	}
	if !out.FastForwardOnly {
		out.Warnings = append(out.Warnings, "local branch is ahead of upstream")
	}
	if (status.Available || status.BinaryOutdated) && !out.ExactTargetCommit {
		out.Blocked = true
		out.Warnings = append(out.Warnings, "update target is not an exact commit hash")
	}
	out.OK = !out.Blocked && out.UpstreamBound && out.FastForwardOnly && out.WorktreeClean
	if status.Available || status.BinaryOutdated {
		out.OK = out.OK && out.ExactTargetCommit
	}
	return out
}

func remoteNameFromUpstream(upstream string) string {
	upstream = strings.TrimSpace(upstream)
	remoteName, _, ok := strings.Cut(upstream, "/")
	if !ok {
		return ""
	}
	return strings.TrimSpace(remoteName)
}

func classifyGitRemote(raw string) (kind, host, repo string, trusted bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "unknown", "", "", false
	}
	if filepath.IsAbs(raw) || filepath.VolumeName(raw) != "" || strings.HasPrefix(raw, ".") {
		return "local", "", "", false
	}
	if before, after, ok := strings.Cut(raw, ":"); ok && after != "" && !strings.Contains(before, `\`) {
		if strings.Contains(before, "@") || strings.Contains(before, ".") {
			host = before
			if at := strings.LastIndex(host, "@"); at >= 0 {
				host = host[at+1:]
			}
			host = normalizeGitRemoteHost(host)
			repo = normalizeGitRemotePath(after)
			return "network", host, repo, isTrustedUpdateRemote(host, repo)
		}
	}
	if parsed, err := url.Parse(raw); err == nil && parsed.Scheme != "" {
		switch strings.ToLower(parsed.Scheme) {
		case "file":
			return "local", "", "", false
		case "http", "https", "ssh", "git":
			host = normalizeGitRemoteHost(parsed.Host)
			repo = normalizeGitRemotePath(parsed.Path)
			return "network", host, repo, isTrustedUpdateRemote(host, repo)
		default:
			return "unknown", "", "", false
		}
	}
	return "local", "", "", false
}

func normalizeGitRemoteHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	if strings.Contains(host, ":") && !strings.Contains(host, "]") {
		host = strings.Split(host, ":")[0]
	}
	return strings.Trim(host, "[]")
}

func normalizeGitRemotePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, ".git")
	return strings.ToLower(path)
}

func isTrustedUpdateRemote(host, repo string) bool {
	return host == trustedUpdateHost && repo == trustedUpdateRepo
}

func isFullCommitHash(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 40 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func cleanCommandError(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

func gitFailureMessage(action, detail, proxyURL string) string {
	lower := strings.ToLower(detail)
	proxyHint := "请检查服务器到 GitHub 的网络连通性"
	if strings.TrimSpace(proxyURL) == "" {
		proxyHint += "，或在设置页配置服务端可访问的更新代理"
	} else {
		proxyHint += "，并确认当前更新代理可由服务端访问"
	}
	switch {
	case strings.Contains(lower, "gnutls") || strings.Contains(lower, "handshake") || strings.Contains(lower, "tls"):
		return fmt.Sprintf("Git %s 失败：GitHub TLS 连接被中断。%s。", action, proxyHint)
	case strings.Contains(lower, "could not resolve host") || strings.Contains(lower, "name resolution"):
		return fmt.Sprintf("Git %s 失败：服务器无法解析 GitHub 域名。请检查 DNS、网络或更新代理。", action)
	case strings.Contains(lower, "connection timed out") || strings.Contains(lower, "failed to connect") || strings.Contains(lower, "connection refused"):
		return fmt.Sprintf("Git %s 失败：服务器连接 GitHub 超时或被拒绝。%s。", action, proxyHint)
	case strings.Contains(lower, "authentication failed") || strings.Contains(lower, "could not read username"):
		return fmt.Sprintf("Git %s 失败：远端仓库认证失败。请检查仓库地址、访问权限或凭据配置。", action)
	default:
		return fmt.Sprintf("Git %s 失败。%s；点击详情可查看 Git 原始错误。", action, proxyHint)
	}
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
