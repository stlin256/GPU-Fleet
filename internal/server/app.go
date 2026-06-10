package server

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gpufleet/internal/auth"
	"gpufleet/internal/model"
	"gpufleet/internal/version"
)

type Config struct {
	Addr                   string
	AddrExplicit           bool
	DataDir                string
	MinFreeBytes           uint64
	Retention              time.Duration
	BootstrapDeviceID      string
	BootstrapSecret        string
	AdminPassword          string
	WebDir                 string
	RepoDir                string
	DisableUpdateMonitor   bool
	DisableTelemetry       bool
	TelemetryEndpoint      string
	TelemetryInterval      time.Duration
	AgentUpdateManifestURL string
	AgentUpdatePublicKey   string
}

type App struct {
	config                Config
	meta                  *MetadataStore
	metrics               *MetricsStore
	processes             *ProcessStore
	nonces                *NonceStore
	sessions              *SessionStore
	loginRate             *RateLimiter
	loginGuard            *LoginGuard
	agentRate             *RateLimiter
	agentAuthCompatMu     sync.Mutex
	legacyAgentAuthAudit  map[string]time.Time
	updateMu              sync.Mutex
	updateStatusCache     updateStatus
	updateStatusCacheOK   bool
	updateMonitorWake     chan struct{}
	updateBuildServer     updateBuildFunc
	updateScheduleRestart updateRestartFunc
	updateExit            func()
	logger                *log.Logger
	scheme                string
	startedAt             time.Time
}

const webSessionTTL = 30 * 24 * time.Hour

const (
	strictCSPHeader   = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'"
	fallbackCSPHeader = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'"
)

func NewApp(config Config, logger *log.Logger) (*App, string, error) {
	if config.Addr == "" {
		config.Addr = "127.0.0.1:8080"
	}
	if config.DataDir == "" {
		config.DataDir = "data"
	}
	if config.MinFreeBytes == 0 {
		config.MinFreeBytes = 800 * 1024 * 1024
	}
	if config.Retention == 0 {
		config.Retention = 30 * 24 * time.Hour
	}
	if config.WebDir == "" {
		config.WebDir = "web/dist"
	}
	if config.TelemetryEndpoint == "" {
		config.TelemetryEndpoint = defaultTelemetryEndpoint
	}
	if config.TelemetryInterval == 0 {
		config.TelemetryInterval = telemetryDefaultInterval
	}
	if config.RepoDir == "" {
		if wd, err := os.Getwd(); err == nil {
			config.RepoDir = wd
		} else {
			config.RepoDir = "."
		}
	}
	if logger == nil {
		logger = log.Default()
	}

	meta, generatedPassword, err := OpenMetadataStore(
		filepath.Join(config.DataDir, "metadata.json"),
		config.AdminPassword,
		config.BootstrapDeviceID,
		config.BootstrapSecret,
	)
	if err != nil {
		return nil, "", err
	}
	if !config.AddrExplicit {
		if saved := meta.ServiceConfig(); saved.Addr != "" {
			config.Addr = saved.Addr
		}
	}
	if err := meta.EnsureServiceConfig(config.Addr); err != nil {
		return nil, "", err
	}
	if saved := meta.ServiceConfig(); saved.MinFreeBytes > 0 {
		config.MinFreeBytes = saved.MinFreeBytes
	} else if _, err := meta.UpdateMinFreeBytes(config.MinFreeBytes); err != nil {
		return nil, "", err
	}
	metrics, err := NewMetricsStore(filepath.Join(config.DataDir, "metrics"), config.MinFreeBytes, config.Retention)
	if err != nil {
		return nil, "", err
	}
	processes, err := NewProcessStore(filepath.Join(config.DataDir, "processes.json"))
	if err != nil {
		return nil, "", err
	}

	scheme := "http"
	if certFile, keyFile, ok := meta.CertificateFiles(); ok && meta.SetupComplete() {
		if _, err := os.Stat(certFile); err == nil {
			if _, err := os.Stat(keyFile); err == nil {
				scheme = "https"
			}
		}
	}

	app := &App{
		config:                config,
		meta:                  meta,
		metrics:               metrics,
		processes:             processes,
		nonces:                NewNonceStore(10 * time.Minute),
		sessions:              NewSessionStore(webSessionTTL, meta),
		loginRate:             NewRateLimiter(10, time.Minute),
		loginGuard:            NewLoginGuard(5, 30*time.Minute, 5*time.Minute, time.Hour),
		agentRate:             NewRateLimiter(240, time.Minute),
		legacyAgentAuthAudit:  map[string]time.Time{},
		updateBuildServer:     defaultBuildServerForUpdate,
		updateScheduleRestart: defaultScheduleRestartAfterUpdate,
		updateMonitorWake:     make(chan struct{}, 1),
		updateExit:            func() { os.Exit(0) },
		logger:                logger,
		scheme:                scheme,
		startedAt:             time.Now().UTC(),
	}
	if app.schedulePendingExecutableRecovery() {
		return app, generatedPassword, nil
	}
	if !config.DisableUpdateMonitor {
		app.startUpdateMonitorLoop()
	}
	if !config.DisableTelemetry && config.TelemetryEndpoint != "" {
		app.startTelemetryLoop()
	}
	return app, generatedPassword, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleIndex)
	mux.HandleFunc("/api/v1/setup/status", a.handleSetupStatus)
	mux.HandleFunc("/api/v1/setup/apply", a.handleSetupApply)
	mux.HandleFunc("/api/v1/guest/status", a.handleGuestStatus)
	mux.HandleFunc("/api/v1/guest/overview", a.handleGuestOverview)
	mux.HandleFunc("/api/v1/guest/gpus/", a.handleGuestGPUSeries)
	mux.HandleFunc("/api/v1/auth/login", a.handleLogin)
	mux.HandleFunc("/api/v1/auth/logout", a.handleLogout)
	mux.HandleFunc("/api/v1/version", a.requireSession(a.handleVersion))
	mux.HandleFunc("/api/v1/overview", a.requireSession(a.handleOverview))
	mux.HandleFunc("/api/v1/devices", a.requireSession(a.handleDevices))
	mux.HandleFunc("/api/v1/gpus/", a.requireSession(a.handleGPUSeries))
	mux.HandleFunc("/api/v1/stats/gpu-utilization", a.requireSession(a.handleGPUStats))
	mux.HandleFunc("/api/v1/energy/summary", a.requireSession(a.handleEnergySummary))
	mux.HandleFunc("/api/v1/processes/latest", a.requireSession(a.handleLatestProcesses))
	mux.HandleFunc("/api/v1/admin/setup/reopen", a.requireSession(a.handleSetupReopen))
	mux.HandleFunc("/api/v1/admin/setup/apply", a.requireSession(a.handleSetupApplyAuthenticated))
	mux.HandleFunc("/api/v1/admin/password", a.requireSession(a.handleAdminPassword))
	mux.HandleFunc("/api/v1/admin/server-config", a.requireSession(a.handleAdminServerConfig))
	mux.HandleFunc("/api/v1/admin/language", a.requireSession(a.handleAdminLanguage))
	mux.HandleFunc("/api/v1/admin/guest", a.requireSession(a.handleAdminGuest))
	mux.HandleFunc("/api/v1/admin/guest/visits", a.requireSession(a.handleAdminGuestVisits))
	mux.HandleFunc("/api/v1/admin/update/proxy", a.requireSession(a.handleAdminUpdateProxy))
	mux.HandleFunc("/api/v1/admin/update-proxy", a.requireSession(a.handleAdminUpdateProxy))
	mux.HandleFunc("/api/v1/admin/update/notice", a.requireSession(a.handleUpdateNotice))
	mux.HandleFunc("/api/v1/admin/certificate", a.requireSession(a.handleAdminCertificate))
	mux.HandleFunc("/api/v1/admin/restart", a.requireSession(a.handleAdminRestart))
	mux.HandleFunc("/api/v1/admin/database/download", a.requireSession(a.handleDatabaseDownload))
	mux.HandleFunc("/api/v1/admin/diagnostics/download", a.requireSession(a.handleDiagnosticsDownload))
	mux.HandleFunc("/api/v1/admin/update/status", a.requireSession(a.handleUpdateStatus))
	mux.HandleFunc("/api/v1/admin/update/apply", a.requireSession(a.handleUpdateApply))
	mux.HandleFunc("/api/v1/admin/devices", a.requireSession(a.handleCreateDevice))
	mux.HandleFunc("/api/v1/admin/devices/", a.requireSession(a.handleAdminDeviceAction))
	mux.HandleFunc("/api/v1/agent/heartbeat", a.handleAgentHeartbeat)
	mux.HandleFunc("/api/v1/agent/samples", a.handleAgentSamples)
	mux.HandleFunc("/api/v1/agent/process-snapshots", a.handleAgentProcesses)
	mux.HandleFunc("/api/v1/agent/config", a.handleAgentConfig)
	mux.HandleFunc("/api/v1/agent/update-policy", a.handleAgentUpdatePolicy)
	mux.HandleFunc("/api/v1/agent/update-events", a.handleAgentUpdateEvents)
	return securityHeaders(mux)
}

func (a *App) ListenAndServe() error {
	server := &http.Server{
		Addr:              a.config.Addr,
		Handler:           a.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if a.scheme == "https" {
		certFile, keyFile, ok := a.meta.CertificateFiles()
		if !ok {
			return errors.New("HTTPS is enabled but certificate files are missing")
		}
		if _, err := os.Stat(certFile); err == nil {
			if _, err := os.Stat(keyFile); err == nil {
				return server.ListenAndServeTLS(certFile, keyFile)
			}
		}
	}
	return server.ListenAndServe()
}

func (a *App) Scheme() string {
	return a.scheme
}

func (a *App) Addr() string {
	return a.config.Addr
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if a.serveWebDist(w, r) {
		return
	}
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Security-Policy", fallbackCSPHeader)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, dashboardHTML)
}

func (a *App) serveWebDist(w http.ResponseWriter, r *http.Request) bool {
	webRoot, err := filepath.Abs(a.config.WebDir)
	if err != nil {
		return false
	}
	index := filepath.Join(webRoot, "index.html")
	if _, err := os.Stat(index); err != nil {
		return false
	}

	cleanPath := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if cleanPath == "." || cleanPath == string(filepath.Separator) {
		http.ServeFile(w, r, index)
		return true
	}

	target := filepath.Join(webRoot, cleanPath)
	rel, err := filepath.Rel(webRoot, target)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		http.NotFound(w, r)
		return true
	}
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		http.ServeFile(w, r, target)
		return true
	}

	if strings.Contains(filepath.Base(cleanPath), ".") {
		http.NotFound(w, r)
		return true
	}
	http.ServeFile(w, r, index)
	return true
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	now := time.Now()
	loginKey := clientIP(r)
	if locked := a.loginGuard.Check(loginKey, now); locked.Locked {
		_ = a.meta.AddAudit("login_lockout_blocked", "blocked login while source is locked")
		writeRetryAfterError(w, http.StatusTooManyRequests, "too many login attempts; retry later", locked.RetryFor)
		return
	}
	if allowed, retryFor := a.loginRate.AllowWithRetry(loginKey, now); !allowed {
		writeRetryAfterError(w, http.StatusTooManyRequests, "too many login attempts; retry later", retryFor)
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body, 1<<20); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !a.meta.VerifyAdmin(body.Password) {
		_ = a.meta.AddAudit("login_failed", "admin login failed")
		if result := a.loginGuard.RecordFailure(loginKey, now); result.Locked {
			_ = a.meta.AddAudit("login_lockout_started", fmt.Sprintf("locked login source after repeated failures; level %d", result.LockLevel))
			writeRetryAfterError(w, http.StatusTooManyRequests, "too many login attempts; retry later", result.RetryFor)
			return
		}
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	a.loginGuard.RecordSuccess(loginKey)
	if err := a.sessions.Create(w, requestScheme(r, a.scheme) == "https"); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = a.meta.AddAudit("login_success", "admin login succeeded")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, a.setupStatus(r))
}

func (a *App) handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, a.releaseInfo())
}

func (a *App) releaseInfo() version.ReleaseInfo {
	for _, path := range a.changelogPaths() {
		if entries, err := version.ChangelogFromFile(path); err == nil && len(entries) > 0 {
			info := version.Current()
			info.Changelog = entries
			return info
		}
	}
	return version.Current()
}

func (a *App) changelogPaths() []string {
	seen := map[string]bool{}
	var paths []string
	add := func(dir string) {
		if strings.TrimSpace(dir) == "" {
			return
		}
		path, err := filepath.Abs(filepath.Join(dir, "CHANGELOG.md"))
		if err != nil || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	add(a.config.RepoDir)
	if wd, err := os.Getwd(); err == nil {
		add(wd)
	}
	if exe, err := os.Executable(); err == nil {
		add(filepath.Dir(exe))
	}
	add("/opt/gpufleet/repo")
	add("/opt/gpufleet")
	return paths
}

func (a *App) handleSetupApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if a.meta.HasAdminPassword() || a.meta.SetupComplete() {
		writeError(w, http.StatusForbidden, "setup is already completed")
		return
	}
	var body struct {
		Password       string `json:"password"`
		Port           int    `json:"port"`
		Language       string `json:"language"`
		CertificatePEM string `json:"certificate_pem"`
		PrivateKeyPEM  string `json:"private_key_pem"`
	}
	if err := decodeJSON(r, &body, 4<<20); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	config, err := a.meta.CompleteInitialSetup(
		body.Password,
		body.Port,
		body.Language,
		[]byte(strings.TrimSpace(body.CertificatePEM)),
		[]byte(strings.TrimSpace(body.PrivateKeyPEM)),
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"service":          a.serviceStatusFromConfig(config, r),
		"restart_required": a.restartRequired(config),
	})
}

func (a *App) handleSetupReopen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	_ = a.meta.AddAudit("setup_reopened", "reopened setup wizard from settings")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "setup": a.setupStatus(r)})
}

func (a *App) handleSetupApplyAuthenticated(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Password       string `json:"password"`
		Port           int    `json:"port"`
		Language       string `json:"language"`
		CertificatePEM string `json:"certificate_pem"`
		PrivateKeyPEM  string `json:"private_key_pem"`
	}
	if err := decodeJSON(r, &body, 4<<20); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	config, err := a.meta.ReconfigureSetup(
		body.Password,
		body.Port,
		body.Language,
		[]byte(strings.TrimSpace(body.CertificatePEM)),
		[]byte(strings.TrimSpace(body.PrivateKeyPEM)),
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"service":          a.serviceStatusFromConfig(config, r),
		"restart_required": a.restartRequired(config),
	})
}

func (a *App) handleAdminPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
		NextPassword    string `json:"next_password"`
	}
	if err := decodeJSON(r, &body, 1<<20); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.meta.UpdatePassword(body.CurrentPassword, body.NextPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleAdminServerConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Port                   int                      `json:"port"`
		MinFreeMB              *int                     `json:"min_free_mb"`
		AutoUpdateEnabled      *bool                    `json:"auto_update_enabled"`
		LegacyAgentAuthEnabled *bool                    `json:"legacy_agent_auth_enabled"`
		AgentUpdate            *model.AgentUpdatePolicy `json:"agent_update"`
		EnergyPricePerKWh      *float64                 `json:"energy_price_per_kwh"`
		EnergyCurrency         *string                  `json:"energy_currency"`
		ThermalHotCelsius      *float64                 `json:"thermal_hot_celsius"`
		IdleUtilizationPercent *float64                 `json:"idle_utilization_percent"`
		IdlePowerWatts         *float64                 `json:"idle_power_watts"`
	}
	if err := decodeJSON(r, &body, 1<<20); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	config := a.meta.ServiceConfig()
	var err error
	if body.Port > 0 {
		config, err = a.meta.UpdateServicePort(body.Port)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if body.MinFreeMB != nil {
		if *body.MinFreeMB < 100 {
			writeError(w, http.StatusBadRequest, "minimum disk reserve must be at least 100 MiB")
			return
		}
		minFreeBytes := uint64(*body.MinFreeMB) * 1024 * 1024
		config, err = a.meta.UpdateMinFreeBytes(minFreeBytes)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		a.config.MinFreeBytes = minFreeBytes
		a.metrics.SetMinFreeBytes(minFreeBytes)
	}
	if body.AutoUpdateEnabled != nil {
		config, err = a.meta.UpdateAutoUpdateEnabled(*body.AutoUpdateEnabled)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		a.wakeUpdateMonitor()
	}
	if body.LegacyAgentAuthEnabled != nil {
		config, err = a.meta.UpdateLegacyAgentAuthEnabled(*body.LegacyAgentAuthEnabled)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if body.AgentUpdate != nil {
		config, err = a.meta.UpdateAgentUpdatePolicy(a.withAgentUpdateDefaults(*body.AgentUpdate))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if body.EnergyPricePerKWh != nil || body.EnergyCurrency != nil || body.ThermalHotCelsius != nil || body.IdleUtilizationPercent != nil || body.IdlePowerWatts != nil {
		settings := config.EnergySettings()
		if body.EnergyPricePerKWh != nil {
			settings.EnergyPricePerKWh = *body.EnergyPricePerKWh
		}
		if body.EnergyCurrency != nil {
			settings.EnergyCurrency = *body.EnergyCurrency
		}
		if body.ThermalHotCelsius != nil {
			settings.ThermalHotCelsius = *body.ThermalHotCelsius
		}
		if body.IdleUtilizationPercent != nil {
			settings.IdleUtilizationPercent = *body.IdleUtilizationPercent
		}
		if body.IdlePowerWatts != nil {
			settings.IdlePowerWatts = *body.IdlePowerWatts
		}
		config, err = a.meta.UpdateEnergySettings(settings)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"service":          a.serviceStatusFromConfig(config, r),
		"restart_required": a.restartRequired(config),
	})
}

func (a *App) withAgentUpdateDefaults(policy model.AgentUpdatePolicy) model.AgentUpdatePolicy {
	if !policy.Enabled {
		return policy
	}
	current := a.meta.ServiceConfig().AgentUpdate
	if strings.TrimSpace(policy.ManifestURL) == "" {
		if strings.TrimSpace(current.ManifestURL) != "" {
			policy.ManifestURL = current.ManifestURL
		} else {
			policy.ManifestURL = a.config.AgentUpdateManifestURL
		}
	}
	if strings.TrimSpace(policy.PublicKey) == "" {
		if strings.TrimSpace(current.PublicKey) != "" {
			policy.PublicKey = current.PublicKey
		} else {
			policy.PublicKey = a.config.AgentUpdatePublicKey
		}
	}
	return policy
}

func (a *App) handleAdminLanguage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Language string `json:"language"`
	}
	if err := decodeJSON(r, &body, 1<<20); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	config, err := a.meta.UpdateLanguage(body.Language)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"service":          a.serviceStatusFromConfig(config, r),
		"restart_required": a.restartRequired(config),
	})
}

func (a *App) handleAdminGuest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &body, 1<<20); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	config, err := a.meta.UpdateGuestEnabled(body.Enabled)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"service":          a.serviceStatusFromConfig(config, r),
		"restart_required": false,
	})
}

func (a *App) handleAdminGuestVisits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"visits": a.meta.GuestVisits(100),
	})
}

func (a *App) handleAdminUpdateProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ProxyURL string `json:"proxy_url"`
	}
	if err := decodeJSON(r, &body, 1<<20); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	config, err := a.meta.UpdateProxy(body.ProxyURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"service":          a.serviceStatusFromConfig(config, r),
		"restart_required": false,
	})
}

func (a *App) handleAdminCertificate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		CertificatePEM string `json:"certificate_pem"`
		PrivateKeyPEM  string `json:"private_key_pem"`
	}
	if err := decodeJSON(r, &body, 4<<20); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	config, err := a.meta.SaveCertificate([]byte(strings.TrimSpace(body.CertificatePEM)), []byte(strings.TrimSpace(body.PrivateKeyPEM)))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = a.addAuditForRequest(r, "service_certificate_uploaded", "uploaded HTTPS certificate", "")
	restartRequired := true
	restartAt, err := a.scheduleServiceRestart()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("schedule restart failed: %s", limitText(err.Error(), 500)))
		return
	}
	_ = a.meta.AddAudit("certificate_restart_scheduled", "saved HTTPS certificate and scheduled automatic restart")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"service":          a.serviceStatusFromConfig(config, r),
		"restart_required": restartRequired,
		"restarting":       true,
		"restart_at":       restartAt,
	})
}

func (a *App) handleAdminRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	restartAt, err := a.scheduleServiceRestart()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("schedule restart failed: %s", limitText(err.Error(), 500)))
		return
	}
	_ = a.addAuditForRequest(r, "service_restart_scheduled", "manual service restart scheduled from settings", "")
	_ = a.meta.AddAudit("service_restart_scheduled", "manual service restart scheduled from settings")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"service":          a.serviceStatusFromConfig(a.meta.ServiceConfig(), r),
		"restart_required": true,
		"restarting":       true,
		"restart_at":       restartAt,
	})
}

func (a *App) scheduleServiceRestart() (time.Time, error) {
	exePath, err := currentExecutablePath()
	if err != nil {
		return time.Time{}, err
	}
	restartAt := time.Now().UTC().Add(updateRestartDelay)
	restartReq := updateRestartRequest{
		CurrentExe:        exePath,
		Args:              append([]string(nil), os.Args[1:]...),
		WorkDir:           mustGetwd(),
		PID:               os.Getpid(),
		RestartAt:         restartAt,
		ReplaceExecutable: false,
		ManagedBySystemd:  runningUnderSystemd(),
	}
	if err := a.updateScheduleRestart(restartReq); err != nil {
		return time.Time{}, err
	}
	go func() {
		time.Sleep(updateRestartDelay)
		if a.updateExit != nil {
			a.updateExit()
			return
		}
		os.Exit(0)
	}()
	return restartAt, nil
}

func (a *App) handleDatabaseDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "gpufleet-data-"+time.Now().UTC().Format("20060102-150405")+".zip"))
	archive := zip.NewWriter(w)
	defer archive.Close()

	dataRoot, err := filepath.Abs(a.config.DataDir)
	if err != nil {
		a.logger.Printf("database download failed: %v", err)
		return
	}
	err = filepath.WalkDir(dataRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dataRoot, path)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if shouldSkipDatabaseArchiveFile(relSlash) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relSlash
		header.Method = zip.Deflate
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})
	if err != nil {
		a.logger.Printf("database download failed: %v", err)
	}
}

func (a *App) handleDiagnosticsDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	report := a.diagnosticsReport(r)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "gpufleet-diagnostics-"+time.Now().UTC().Format("20060102-150405")+".zip"))
	archive := zip.NewWriter(w)
	defer archive.Close()
	if err := addJSONZipEntry(archive, "diagnostics.json", report); err != nil {
		a.logger.Printf("diagnostics download failed: %v", err)
	}
}

func addJSONZipEntry(archive *zip.Writer, name string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	header := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	header.SetModTime(time.Now().UTC())
	writer, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = writer.Write(raw)
	return err
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.sessions.Clear(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleOverview(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.overviewResponse(r, false))
}

func (a *App) handleGuestStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"guest_enabled": a.meta.ServiceConfig().GuestEnabled,
		"service":       a.guestServiceStatus(),
	})
}

func (a *App) handleGuestOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !a.meta.ServiceConfig().GuestEnabled {
		writeError(w, http.StatusForbidden, "guest access disabled")
		return
	}
	now := time.Now().UTC()
	_ = a.meta.RecordGuestVisit(GuestVisit{
		At:          now,
		RemoteIP:    clientIP(r),
		UserAgent:   r.UserAgent(),
		Path:        r.URL.Path,
		Fingerprint: r.Header.Get("X-GPUFleet-Guest-Fingerprint"),
		Language:    r.Header.Get("X-GPUFleet-Guest-Language"),
		Platform:    r.Header.Get("X-GPUFleet-Guest-Platform"),
		Screen:      r.Header.Get("X-GPUFleet-Guest-Screen"),
		Timezone:    r.Header.Get("X-GPUFleet-Guest-Timezone"),
	})
	writeJSON(w, http.StatusOK, a.overviewResponse(r, true))
}

func (a *App) handleGuestGPUSeries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !a.meta.ServiceConfig().GuestEnabled {
		writeError(w, http.StatusForbidden, "guest access disabled")
		return
	}
	if !strings.HasSuffix(r.URL.Path, "/series") {
		http.NotFound(w, r)
		return
	}
	gpuID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/guest/gpus/"), "/series")
	if gpuID == "" {
		writeError(w, http.StatusBadRequest, "missing gpu id")
		return
	}
	guestDeviceID := r.URL.Query().Get("device_id")
	if guestDeviceID == "" {
		writeError(w, http.StatusBadRequest, "missing device_id")
		return
	}
	deviceID, ok := a.realDeviceIDForGuest(guestDeviceID)
	if !ok {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}
	hours := parseHours(r, 1)
	points, err := a.gpuSeries(deviceID, gpuID, hours)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, points)
}

func (a *App) gpuSeries(deviceID, gpuID string, hours int) ([]SeriesPoint, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	if hours > 1 {
		return a.metrics.SeriesRollup(deviceID, gpuID, since)
	}
	return a.metrics.Series(deviceID, gpuID, since)
}

func (a *App) overviewResponse(r *http.Request, guest bool) overviewResponse {
	devices := a.meta.ListDevices()
	diskStatus, _ := a.metrics.DiskStatus()
	databaseSize := databaseSizeBytes(a.config.DataDir)
	metricStoredDays := a.metrics.StoredDays()

	deviceViews := make([]deviceView, 0, len(devices))
	deviceIDs := make(map[string]bool, len(devices))
	guestDeviceIDs := guestDeviceIDMap(devices)
	now := time.Now().UTC()
	online := 0
	for _, device := range devices {
		deviceIDs[device.ID] = true
		guestID := guestDeviceIDs[device.ID]
		alias := device.Alias
		if guest && alias == "" {
			alias = fmt.Sprintf("Device %d", len(deviceViews)+1)
		}
		status := "offline"
		if device.Enabled && !device.LastSeenAt.IsZero() && now.Sub(device.LastSeenAt) <= 45*time.Second {
			status = "online"
			online++
		}
		deviceViews = append(deviceViews, deviceView{
			ID:           chooseString(!guest, device.ID),
			Alias:        alias,
			Enabled:      device.Enabled,
			Status:       status,
			Hostname:     chooseString(!guest, device.Hostname),
			OS:           chooseString(!guest, device.OS),
			OSVersion:    chooseString(!guest, device.OSVersion),
			AgentVersion: chooseString(!guest, device.AgentVersion),
			GPUCount:     device.GPUCount,
			LastSeenAt:   device.LastSeenAt,
			LastSampleAt: device.LastSampleAt,
			LastError:    chooseString(!guest, device.LastError),
		})
		if guest {
			deviceViews[len(deviceViews)-1].ID = guestID
		}
	}
	sort.Slice(deviceViews, func(i, j int) bool { return deviceViews[i].Alias < deviceViews[j].Alias })

	latest := a.metrics.Latest()
	filteredLatest := make([]StoredGPU, 0, len(latest))
	totalUtil := 0.0
	utilCount := 0
	usedMem := uint64(0)
	totalMem := uint64(0)
	totalPower := 0.0
	hot := 0
	for _, item := range latest {
		if !deviceIDs[item.DeviceID] {
			continue
		}
		if guest {
			item.DeviceID = guestDeviceIDs[item.DeviceID]
			item.GPU.UUIDHash = ""
			item.GPU.VBIOSVersion = ""
			item.GPU.DriverVersion = ""
			item.GPU.CollectionError = ""
		}
		filteredLatest = append(filteredLatest, item)
		if item.GPU.UtilizationGPUPercent != nil {
			totalUtil += *item.GPU.UtilizationGPUPercent
			utilCount++
		}
		usedMem += item.GPU.MemoryUsedBytes
		totalMem += item.GPU.MemoryTotalBytes
		if item.GPU.PowerDrawWatts != nil {
			totalPower += *item.GPU.PowerDrawWatts
		}
		if item.GPU.TemperatureCelsius != nil && *item.GPU.TemperatureCelsius >= 85 {
			hot++
		}
	}
	avgUtil := 0.0
	if utilCount > 0 {
		avgUtil = totalUtil / float64(utilCount)
	}

	response := overviewResponse{
		ServerTime:         now,
		DeviceCount:        len(devices),
		OnlineDeviceCount:  online,
		GPUCount:           len(filteredLatest),
		AverageUtilization: avgUtil,
		MemoryUsedBytes:    usedMem,
		MemoryTotalBytes:   totalMem,
		PowerDrawWatts:     totalPower,
		HotGPUCount:        hot,
		Disk:               diskStatus,
		Devices:            deviceViews,
		LatestGPUs:         filteredLatest,
		LatestProcesses:    nil,
		RetentionHours:     int(a.config.Retention.Hours()),
		MetricStoredDays:   metricStoredDays,
		MinFreeSpaceBytes:  a.config.MinFreeBytes,
		DatabaseSizeBytes:  databaseSize,
		SetupComplete:      a.meta.SetupComplete(),
		Guest:              guest,
	}
	if !guest {
		response.Service = a.serviceStatus(r)
		response.LatestProcesses = filterProcessesByDevices(a.processes.Latest("", ""), deviceIDs)
	} else {
		response.Service = serviceStatusFromGuest(a.guestServiceStatus())
	}
	return response
}

func (a *App) diagnosticsReport(r *http.Request) diagnosticsReport {
	now := time.Now().UTC()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	service := a.serviceStatus(r)
	service.UpdateProxy = redactURLCredentials(service.UpdateProxy)
	telemetry := a.diagnosticsTelemetry()

	dataDir := a.config.DataDir
	if abs, err := filepath.Abs(a.config.DataDir); err == nil {
		dataDir = abs
	}
	diskStatus, diskErr := a.metrics.DiskStatus()
	storage := diagnosticsStorage{
		DataDir:             dataDir,
		DatabaseSizeBytes:   databaseSizeBytes(a.config.DataDir),
		RetentionHours:      int(a.config.Retention.Hours()),
		MetricStoredDays:    a.metrics.StoredDays(),
		MinFreeSpaceBytes:   a.config.MinFreeBytes,
		Disk:                diskStatus,
		MetricSegmentCount:  0,
		MetricSegmentOldest: time.Time{},
		MetricSegmentNewest: time.Time{},
	}
	if diskErr != nil {
		storage.DiskError = diskErr.Error()
	}
	segments := a.metricSegmentDiagnostics()
	storage.MetricSegmentCount = segments.Count
	storage.MetricSegmentOldest = segments.Oldest
	storage.MetricSegmentNewest = segments.Newest
	storage.MetricSegmentError = segments.Error

	processes := a.processDiagnostics(a.processes.Latest("", ""))
	devices := diagnosticsDevices(a.meta.ListDevices(), now)
	gpus := diagnosticsGPUs(a.metrics.Latest())

	return diagnosticsReport{
		Product:     version.Product,
		Version:     version.Version,
		Commit:      version.Commit,
		BuildTime:   version.BuildTime,
		GeneratedAt: now,
		UptimeSeconds: func() int64 {
			if a.startedAt.IsZero() {
				return 0
			}
			return int64(now.Sub(a.startedAt).Seconds())
		}(),
		Runtime: diagnosticsRuntime{
			GoVersion:        runtime.Version(),
			GOOS:             runtime.GOOS,
			GOARCH:           runtime.GOARCH,
			NumCPU:           runtime.NumCPU(),
			NumGoroutine:     runtime.NumGoroutine(),
			MemoryAllocBytes: mem.Alloc,
			MemorySysBytes:   mem.Sys,
			HeapAllocBytes:   mem.HeapAlloc,
			HeapObjects:      mem.HeapObjects,
			NextGCBytes:      mem.NextGC,
			NumGC:            mem.NumGC,
		},
		Service:   service,
		Telemetry: telemetry,
		Storage:   storage,
		Devices:   devices,
		GPUs:      gpus,
		Processes: processes,
		Update:    a.cachedDiagnosticsUpdateStatus(),
		Audit:     diagnosticsAuditEvents(a.meta.RecentAuditEvents(100)),
	}
}

func (a *App) diagnosticsTelemetry() diagnosticsTelemetry {
	state := a.meta.TelemetryState()
	proxy := a.meta.ServiceConfig().UpdateProxy
	lastError := ""
	if strings.TrimSpace(state.LastError) != "" {
		lastError = redactTelemetryError(state.LastError, proxy)
	}
	return diagnosticsTelemetry{
		Enabled:         !a.config.DisableTelemetry && strings.TrimSpace(a.config.TelemetryEndpoint) != "",
		Endpoint:        redactURLCredentials(a.config.TelemetryEndpoint),
		ProxyConfigured: strings.TrimSpace(proxy) != "",
		LastReportAt:    state.LastReportAt,
		LastSuccessAt:   state.LastSuccessAt,
		NextReportAfter: state.NextReportAfter,
		LastError:       lastError,
	}
}

func (a *App) metricSegmentDiagnostics() diagnosticsMetricSegments {
	files, err := a.metrics.segmentFiles()
	if err != nil {
		return diagnosticsMetricSegments{Error: err.Error()}
	}
	out := diagnosticsMetricSegments{Count: len(files)}
	for _, path := range files {
		at, ok := segmentStart(path)
		if !ok {
			continue
		}
		if out.Oldest.IsZero() || at.Before(out.Oldest) {
			out.Oldest = at
		}
		end := at.Add(time.Hour)
		if out.Newest.IsZero() || end.After(out.Newest) {
			out.Newest = end
		}
	}
	return out
}

func diagnosticsDevices(devices []Device, now time.Time) []diagnosticsDevice {
	out := make([]diagnosticsDevice, 0, len(devices))
	for _, device := range devices {
		status := "offline"
		if device.Enabled && !device.LastSeenAt.IsZero() && now.Sub(device.LastSeenAt) <= 45*time.Second {
			status = "online"
		}
		out = append(out, diagnosticsDevice{
			ID:           device.ID,
			Alias:        device.Alias,
			Enabled:      device.Enabled,
			Status:       status,
			Hostname:     device.Hostname,
			OS:           device.OS,
			OSVersion:    device.OSVersion,
			AgentVersion: device.AgentVersion,
			GPUCount:     device.GPUCount,
			CreatedAt:    device.CreatedAt,
			LastSeenAt:   device.LastSeenAt,
			LastSampleAt: device.LastSampleAt,
			LastError:    device.LastError,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		left := out[i].Alias
		if left == "" {
			left = out[i].ID
		}
		right := out[j].Alias
		if right == "" {
			right = out[j].ID
		}
		if left == right {
			return out[i].ID < out[j].ID
		}
		return left < right
	})
	return out
}

func diagnosticsGPUs(latest []StoredGPU) []diagnosticsGPU {
	out := make([]diagnosticsGPU, 0, len(latest))
	for _, item := range latest {
		gpu := item.GPU
		out = append(out, diagnosticsGPU{
			DeviceID:              item.DeviceID,
			GPUID:                 gpu.GPUID,
			Timestamp:             item.Timestamp,
			Name:                  gpu.Name,
			MemoryTotalBytes:      gpu.MemoryTotalBytes,
			MemoryUsedBytes:       gpu.MemoryUsedBytes,
			MemoryFreeBytes:       gpu.MemoryFreeBytes,
			MemoryReservedBytes:   gpu.MemoryReservedBytes,
			UtilizationGPUPercent: gpu.UtilizationGPUPercent,
			UtilizationMemPercent: gpu.UtilizationMemPercent,
			TemperatureCelsius:    gpu.TemperatureCelsius,
			TemperatureMemCelsius: gpu.TemperatureMemCelsius,
			PowerDrawWatts:        gpu.PowerDrawWatts,
			PowerLimitWatts:       gpu.PowerLimitWatts,
			FanSpeedPercent:       gpu.FanSpeedPercent,
			GraphicsClockMHz:      gpu.GraphicsClockMHz,
			MemoryClockMHz:        gpu.MemoryClockMHz,
			SMClockMHz:            gpu.SMClockMHz,
			PState:                gpu.PState,
			PCIeLinkGeneration:    gpu.PCIeLinkGeneration,
			PCIeLinkWidth:         gpu.PCIeLinkWidth,
			PCIeLinkGenerationMax: gpu.PCIeLinkGenerationMax,
			PCIeLinkWidthMax:      gpu.PCIeLinkWidthMax,
			ClockThrottleReasons:  gpu.ClockThrottleReasons,
			CollectionError:       gpu.CollectionError,
		})
	}
	return out
}

func (a *App) processDiagnostics(items []StoredProcessSnapshot) diagnosticsProcesses {
	byGPU := map[string]*diagnosticsProcessGPU{}
	for _, item := range items {
		key := item.DeviceID + "/" + item.Process.GPUID
		entry := byGPU[key]
		if entry == nil {
			entry = &diagnosticsProcessGPU{
				DeviceID: item.DeviceID,
				GPUID:    item.Process.GPUID,
			}
			byGPU[key] = entry
		}
		entry.ProcessCount++
		if entry.LastSeenAt.IsZero() || item.Timestamp.After(entry.LastSeenAt) {
			entry.LastSeenAt = item.Timestamp
		}
	}
	out := diagnosticsProcesses{
		TotalProcessCount: len(items),
		ByGPU:             make([]diagnosticsProcessGPU, 0, len(byGPU)),
	}
	for _, entry := range byGPU {
		out.ByGPU = append(out.ByGPU, *entry)
	}
	sort.Slice(out.ByGPU, func(i, j int) bool {
		if out.ByGPU[i].DeviceID == out.ByGPU[j].DeviceID {
			return out.ByGPU[i].GPUID < out.ByGPU[j].GPUID
		}
		return out.ByGPU[i].DeviceID < out.ByGPU[j].DeviceID
	})
	return out
}

func (a *App) cachedDiagnosticsUpdateStatus() *diagnosticsUpdateStatus {
	a.updateMu.Lock()
	if !a.updateStatusCacheOK || a.updateStatusCache.CheckedAt.IsZero() {
		a.updateMu.Unlock()
		return nil
	}
	cached := a.updateStatusCache
	a.updateMu.Unlock()
	return &diagnosticsUpdateStatus{
		Available:      cached.Available,
		Supported:      cached.Supported,
		Dirty:          cached.Dirty,
		Branch:         cached.Branch,
		Remote:         redactURLCredentials(cached.Remote),
		Upstream:       cached.Upstream,
		LocalCommit:    cached.LocalCommit,
		RemoteCommit:   cached.RemoteCommit,
		RunningVersion: cached.RunningVersion,
		RunningCommit:  cached.RunningCommit,
		RunningBuild:   cached.RunningBuild,
		RepoVersion:    cached.RepoVersion,
		BinaryOutdated: cached.BinaryOutdated,
		Behind:         cached.Behind,
		Ahead:          cached.Ahead,
		CheckedAt:      cached.CheckedAt,
		SupplyChain:    cached.SupplyChain,
		Failed:         cached.Failed,
		Message:        redactURLCredentials(cached.Message),
	}
}

func diagnosticsAuditEvents(events []AuditEvent) []diagnosticsAuditEvent {
	out := make([]diagnosticsAuditEvent, 0, len(events))
	for _, event := range events {
		out = append(out, diagnosticsAuditEvent{
			At:        event.At,
			Type:      event.Type,
			Message:   event.Message,
			Actor:     event.Actor,
			RemoteIP:  maskRemoteIP(event.RemoteIP),
			DeviceID:  event.DeviceID,
			RequestID: event.RequestID,
		})
	}
	return out
}

func redactURLCredentials(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User == nil {
		return value
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
		parsed.User = url.UserPassword("redacted", "redacted")
	} else {
		parsed.User = url.User("redacted")
	}
	return parsed.String()
}

func maskRemoteIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	host := value
	if parsedHost, _, err := net.SplitHostPort(value); err == nil {
		host = parsedHost
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "redacted"
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return fmt.Sprintf("%d.%d.%d.x", ipv4[0], ipv4[1], ipv4[2])
	}
	ipv6 := ip.To16()
	if ipv6 == nil {
		return "redacted"
	}
	first := uint16(ipv6[0])<<8 | uint16(ipv6[1])
	second := uint16(ipv6[2])<<8 | uint16(ipv6[3])
	return fmt.Sprintf("%x:%x::x", first, second)
}

func chooseString(allowed bool, value string) string {
	if !allowed {
		return ""
	}
	return value
}

func (a *App) realDeviceIDForGuest(guestDeviceID string) (string, bool) {
	for deviceID, maskedID := range guestDeviceIDMap(a.meta.ListDevices()) {
		if guestDeviceID == maskedID {
			return deviceID, true
		}
	}
	return "", false
}

func guestDeviceIDMap(devices []Device) map[string]string {
	sort.Slice(devices, func(i, j int) bool {
		leftAlias := devices[i].Alias
		if leftAlias == "" {
			leftAlias = devices[i].ID
		}
		rightAlias := devices[j].Alias
		if rightAlias == "" {
			rightAlias = devices[j].ID
		}
		if leftAlias == rightAlias {
			return devices[i].ID < devices[j].ID
		}
		return leftAlias < rightAlias
	})
	out := make(map[string]string, len(devices))
	for index, device := range devices {
		out[device.ID] = fmt.Sprintf("guest-device-%d", index+1)
	}
	return out
}

func databaseSizeBytes(dataDir string) uint64 {
	dataRoot, err := filepath.Abs(dataDir)
	if err != nil {
		return 0
	}
	var total uint64
	_ = filepath.WalkDir(dataRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dataRoot, path)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if shouldSkipDatabaseArchiveFile(relSlash) {
			return nil
		}
		info, err := entry.Info()
		if err == nil && info.Size() > 0 {
			total += uint64(info.Size())
		}
		return nil
	})
	return total
}

func shouldSkipDataFile(relSlash string) bool {
	base := filepath.Base(relSlash)
	if base == "" || strings.HasPrefix(base, ".") {
		return true
	}
	if strings.HasSuffix(base, ".tmp") || strings.HasSuffix(base, ".lock") {
		return true
	}
	return false
}

func shouldSkipDatabaseArchiveFile(relSlash string) bool {
	if shouldSkipDataFile(relSlash) {
		return true
	}
	if strings.HasPrefix(relSlash, "certs/") {
		return true
	}
	return false
}

func (a *App) handleDevices(w http.ResponseWriter, r *http.Request) {
	devices := a.meta.ListDevices()
	out := make([]deviceView, 0, len(devices))
	for _, device := range devices {
		out = append(out, deviceView{
			ID:           device.ID,
			Alias:        device.Alias,
			Enabled:      device.Enabled,
			Hostname:     device.Hostname,
			OS:           device.OS,
			OSVersion:    device.OSVersion,
			AgentVersion: device.AgentVersion,
			GPUCount:     device.GPUCount,
			LastSeenAt:   device.LastSeenAt,
			LastSampleAt: device.LastSampleAt,
			LastError:    device.LastError,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleCreateDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Alias string `json:"alias"`
	}
	if err := decodeJSON(r, &body, 1<<20); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	device, secret, err := a.meta.CreateDevice(strings.TrimSpace(body.Alias))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = a.addAuditForRequest(r, "device_created", fmt.Sprintf("created device %s", device.ID), device.ID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"device": deviceView{ID: device.ID, Alias: device.Alias, Enabled: device.Enabled},
		"secret": secret,
	})
}

func (a *App) handleAdminDeviceAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/devices/"), "/")
	if r.Method == http.MethodDelete {
		if len(parts) != 1 || parts[0] == "" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		device, err := a.meta.DeleteDevice(parts[0])
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		a.metrics.RemoveDevice(device.ID)
		if err := a.processes.RemoveDevice(device.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = a.addAuditForRequest(r, "device_deleted", fmt.Sprintf("deleted device %s", device.ID), device.ID)
		writeJSON(w, http.StatusOK, map[string]any{"device": toDeviceView(device)})
		return
	}
	if r.Method == http.MethodPatch {
		if len(parts) != 1 || parts[0] == "" {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		var body struct {
			Alias string `json:"alias"`
		}
		if err := decodeJSON(r, &body, 1<<20); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		device, err := a.meta.RenameDevice(parts[0], body.Alias)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		_ = a.addAuditForRequest(r, "device_renamed", fmt.Sprintf("renamed device %s", device.ID), device.ID)
		writeJSON(w, http.StatusOK, map[string]any{"device": toDeviceView(device)})
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	deviceID, action := parts[0], parts[1]
	switch action {
	case "enable":
		device, err := a.meta.SetDeviceEnabled(deviceID, true)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		_ = a.addAuditForRequest(r, "device_enabled", fmt.Sprintf("enabled device %s", device.ID), device.ID)
		writeJSON(w, http.StatusOK, map[string]any{"device": toDeviceView(device)})
	case "disable":
		device, err := a.meta.SetDeviceEnabled(deviceID, false)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		_ = a.addAuditForRequest(r, "device_disabled", fmt.Sprintf("disabled device %s", device.ID), device.ID)
		writeJSON(w, http.StatusOK, map[string]any{"device": toDeviceView(device)})
	case "rotate-secret":
		device, secret, err := a.meta.RotateDeviceSecret(deviceID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		_ = a.addAuditForRequest(r, "device_secret_rotated", fmt.Sprintf("rotated device secret for %s", device.ID), device.ID)
		writeJSON(w, http.StatusOK, map[string]any{"device": toDeviceView(device), "secret": secret})
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (a *App) handleGPUSeries(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/series") {
		http.NotFound(w, r)
		return
	}
	gpuID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/gpus/"), "/series")
	if gpuID == "" {
		writeError(w, http.StatusBadRequest, "missing gpu id")
		return
	}
	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		writeError(w, http.StatusBadRequest, "missing device_id")
		return
	}
	hours := parseHours(r, 1)
	points, err := a.gpuSeries(deviceID, gpuID, hours)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, points)
}

func (a *App) handleGPUStats(w http.ResponseWriter, r *http.Request) {
	hours := parseHours(r, 24)
	stats, err := a.metrics.Stats(r.URL.Query().Get("device_id"), time.Now().Add(-time.Duration(hours)*time.Hour))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if r.URL.Query().Get("device_id") == "" {
		stats = filterStatsByDevices(stats, a.deviceIDSet())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"hours": hours,
		"stats": stats,
	})
}

func (a *App) handleLatestProcesses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.processes.Latest(r.URL.Query().Get("device_id"), r.URL.Query().Get("gpu_id")))
}

func (a *App) handleAgentHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	deviceID, body, ok := a.authenticateAgent(w, r)
	if !ok {
		return
	}
	var heartbeat model.Heartbeat
	if err := json.Unmarshal(body, &heartbeat); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	err := a.meta.UpdateHeartbeat(deviceID, func(device *Device) {
		device.AgentVersion = heartbeat.AgentVersion
		device.Hostname = heartbeat.Hostname
		device.OS = heartbeat.OS
		device.OSVersion = heartbeat.OSVersion
		device.GPUCount = heartbeat.GPUCount
		device.LastRemoteAddr = clientIP(r)
		device.LastError = ""
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "server_time": time.Now().UTC()})
}

func (a *App) handleAgentSamples(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	deviceID, body, ok := a.authenticateAgent(w, r)
	if !ok {
		return
	}
	var batch model.SampleBatch
	if err := json.Unmarshal(body, &batch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if batch.DeviceID == "" {
		batch.DeviceID = deviceID
	}
	if batch.DeviceID != deviceID {
		writeError(w, http.StatusBadRequest, "device id mismatch")
		return
	}
	if len(batch.Samples) == 0 {
		writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "accepted_samples": 0})
		return
	}
	if err := a.metrics.AppendBatch(batch); err != nil {
		if errors.Is(err, ErrInsufficientStorage) {
			_ = a.meta.AddAudit("disk_guard", "rejected metrics because free disk space is below limit")
			writeError(w, http.StatusInsufficientStorage, "insufficient storage")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	last := batch.Samples[len(batch.Samples)-1].Timestamp
	_ = a.meta.RecordSample(deviceID, last)
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "accepted_samples": len(batch.Samples)})
}

func (a *App) handleAgentProcesses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	deviceID, body, ok := a.authenticateAgent(w, r)
	if !ok {
		return
	}
	var batch model.ProcessBatch
	if err := json.Unmarshal(body, &batch); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if batch.DeviceID == "" {
		batch.DeviceID = deviceID
	}
	if batch.DeviceID != deviceID {
		writeError(w, http.StatusBadRequest, "device id mismatch")
		return
	}
	if batch.Timestamp.IsZero() {
		batch.Timestamp = time.Now().UTC()
	}
	if err := a.processes.Replace(batch); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "accepted_processes": len(batch.Processes)})
}

func (a *App) handleAgentConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	deviceID, body, ok := a.authenticateAgent(w, r)
	if !ok {
		return
	}
	var report model.AgentConfigReport
	if err := json.Unmarshal(body, &report); err != nil {
		writeError(w, http.StatusBadRequest, "invalid config report")
		return
	}
	report = sanitizeAgentConfigReport(deviceID, report)
	if err := a.meta.RecordAgentConfig(deviceID, report); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true})
}

func (a *App) handleAgentUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	deviceID, _, ok := a.authenticateAgent(w, r)
	if !ok {
		return
	}
	policy := a.agentUpdatePolicyForDevice(deviceID)
	writeJSON(w, http.StatusOK, map[string]any{
		"device_id":   deviceID,
		"policy":      policy,
		"server_time": time.Now().UTC(),
	})
}

func (a *App) agentUpdatePolicyForDevice(deviceID string) model.AgentUpdatePolicy {
	config := a.meta.ServiceConfig()
	policy := config.AgentUpdate
	if !policy.Enabled || policy.Rollout != "canary" {
		return policy
	}
	maxParallel := policy.MaxParallel
	if maxParallel <= 0 {
		maxParallel = 1
	}
	active := 0
	scouts := 0
	devices := a.meta.ListDevices()
	sort.Slice(devices, func(i, j int) bool {
		left := devices[i].Alias
		if left == "" {
			left = devices[i].ID
		}
		right := devices[j].Alias
		if right == "" {
			right = devices[j].ID
		}
		if left == right {
			return devices[i].ID < devices[j].ID
		}
		return left < right
	})
	rolloutTarget := agentUpdateRolloutTarget(devices, policy, config.UpdatedAt)
	for _, device := range devices {
		if !device.Enabled {
			continue
		}
		isScout := false
		if policy.DesiredVersion == "" && scouts < maxParallel {
			isScout = true
			scouts++
		}
		if agentUpdateCompleted(device.UpdateState, policy, config.UpdatedAt, rolloutTarget) {
			if device.ID == deviceID {
				if isScout {
					return policy
				}
				policy.Enabled = false
				return policy
			}
			continue
		}
		if active < maxParallel {
			if device.ID == deviceID {
				return policy
			}
			active++
			continue
		}
		if device.ID == deviceID {
			policy.Enabled = false
			return policy
		}
	}
	policy.Enabled = false
	return policy
}

func agentUpdateRolloutTarget(devices []Device, policy model.AgentUpdatePolicy, policyUpdatedAt time.Time) string {
	if policy.DesiredVersion != "" {
		return strings.TrimPrefix(policy.DesiredVersion, "v")
	}
	target := ""
	for _, device := range devices {
		state := device.UpdateState
		if state == nil || !agentUpdateTerminalState(*state) {
			continue
		}
		if !policyUpdatedAt.IsZero() && state.UpdatedAt.Before(policyUpdatedAt) {
			continue
		}
		if policy.ManifestURL != "" && state.ManifestURL != "" && state.ManifestURL != redactURLCredentials(policy.ManifestURL) {
			continue
		}
		candidate := strings.TrimPrefix(strings.TrimSpace(state.TargetVersion), "v")
		if candidate == "" {
			continue
		}
		if state.Status == "skipped" && strings.TrimPrefix(state.CurrentVersion, "v") != candidate {
			continue
		}
		if target == "" || compareAgentUpdateVersions(candidate, target) > 0 {
			target = candidate
		}
	}
	return target
}

func agentUpdateCompleted(state *model.AgentUpdateState, policy model.AgentUpdatePolicy, policyUpdatedAt time.Time, rolloutTarget string) bool {
	if state == nil {
		return false
	}
	if !policyUpdatedAt.IsZero() && state.UpdatedAt.Before(policyUpdatedAt) {
		return false
	}
	if !agentUpdateTerminalState(*state) {
		return false
	}
	if state.Status == "skipped" {
		if state.TargetVersion == "" || strings.TrimPrefix(state.CurrentVersion, "v") != strings.TrimPrefix(state.TargetVersion, "v") {
			return false
		}
	}
	target := strings.TrimPrefix(policy.DesiredVersion, "v")
	if target == "" {
		target = rolloutTarget
	}
	if target != "" && strings.TrimPrefix(state.TargetVersion, "v") != target {
		return false
	}
	if policy.ManifestURL != "" && state.ManifestURL != "" && state.ManifestURL != redactURLCredentials(policy.ManifestURL) {
		return false
	}
	return true
}

func agentUpdateTerminalState(state model.AgentUpdateState) bool {
	return state.Status == "applied" || state.Status == "available" || state.Status == "skipped"
}

func compareAgentUpdateVersions(left, right string) int {
	leftParts, leftOK := parseAgentUpdateVersion(left)
	rightParts, rightOK := parseAgentUpdateVersion(right)
	if leftOK && rightOK {
		for index := range leftParts {
			if leftParts[index] > rightParts[index] {
				return 1
			}
			if leftParts[index] < rightParts[index] {
				return -1
			}
		}
		return 0
	}
	return strings.Compare(strings.TrimPrefix(left, "v"), strings.TrimPrefix(right, "v"))
}

func parseAgentUpdateVersion(raw string) ([3]int, bool) {
	var out [3]int
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "v"))
	if raw == "" {
		return out, false
	}
	if index := strings.IndexAny(raw, "-+"); index >= 0 {
		raw = raw[:index]
	}
	parts := strings.Split(raw, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return out, false
	}
	for index, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			return out, false
		}
		out[index] = value
	}
	return out, true
}

func (a *App) handleAgentUpdateEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	deviceID, body, ok := a.authenticateAgent(w, r)
	if !ok {
		return
	}
	var event model.AgentUpdateEvent
	if err := json.Unmarshal(body, &event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid update event")
		return
	}
	event = sanitizeAgentUpdateEvent(event)
	if err := a.meta.RecordAgentUpdateEvent(deviceID, event); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true})
}

func sanitizeAgentUpdateEvent(event model.AgentUpdateEvent) model.AgentUpdateEvent {
	event.Status = limitText(strings.TrimSpace(event.Status), 80)
	event.CurrentVersion = limitText(strings.TrimSpace(event.CurrentVersion), 80)
	event.TargetVersion = limitText(strings.TrimSpace(event.TargetVersion), 80)
	event.ManifestURL = limitText(redactURLCredentials(event.ManifestURL), 260)
	event.ArtifactURL = limitText(redactURLCredentials(event.ArtifactURL), 260)
	event.ArtifactSHA256 = limitText(strings.TrimSpace(event.ArtifactSHA256), 96)
	event.Message = limitText(strings.TrimSpace(event.Message), 240)
	return event
}

func sanitizeAgentConfigReport(deviceID string, report model.AgentConfigReport) model.AgentConfigReport {
	report.DeviceID = deviceID
	report.AgentVersion = limitText(strings.TrimSpace(report.AgentVersion), 80)
	report.Hostname = limitText(strings.TrimSpace(report.Hostname), 160)
	report.OS = limitText(strings.TrimSpace(report.OS), 80)
	report.OSVersion = limitText(strings.TrimSpace(report.OSVersion), 160)
	report.Architecture = limitText(strings.TrimSpace(report.Architecture), 80)
	report.Runtime = limitText(strings.TrimSpace(report.Runtime), 120)
	report.ExecutablePath = limitText(strings.TrimSpace(report.ExecutablePath), 260)
	report.WorkingDirectory = limitText(strings.TrimSpace(report.WorkingDirectory), 260)
	report.ServerURL = limitText(redactURLCredentials(report.ServerURL), 260)
	report.NvidiaSMICommand = limitText(strings.TrimSpace(report.NvidiaSMICommand), 160)
	report.NvidiaSMIResolvedPath = limitText(strings.TrimSpace(report.NvidiaSMIResolvedPath), 260)
	report.NvidiaSMIVersion = limitText(strings.TrimSpace(report.NvidiaSMIVersion), 2000)
	report.QueuePath = limitText(strings.TrimSpace(report.QueuePath), 260)
	if len(report.GPUs) > 64 {
		report.GPUs = report.GPUs[:64]
	}
	for index := range report.GPUs {
		gpu := &report.GPUs[index]
		gpu.GPUID = limitText(strings.TrimSpace(gpu.GPUID), 80)
		gpu.UUIDHash = limitText(strings.TrimSpace(gpu.UUIDHash), 96)
		gpu.Name = limitText(strings.TrimSpace(gpu.Name), 160)
		gpu.DriverVersion = limitText(strings.TrimSpace(gpu.DriverVersion), 80)
		gpu.VBIOSVersion = limitText(strings.TrimSpace(gpu.VBIOSVersion), 80)
		gpu.PCIeLinkGeneration = limitText(strings.TrimSpace(gpu.PCIeLinkGeneration), 40)
		gpu.PCIeLinkGenerationMax = limitText(strings.TrimSpace(gpu.PCIeLinkGenerationMax), 40)
		gpu.PCIeLinkWidth = limitText(strings.TrimSpace(gpu.PCIeLinkWidth), 40)
		gpu.PCIeLinkWidthMax = limitText(strings.TrimSpace(gpu.PCIeLinkWidthMax), 40)
		gpu.ComputeMode = limitText(strings.TrimSpace(gpu.ComputeMode), 80)
		gpu.ComputeCapability = limitText(strings.TrimSpace(gpu.ComputeCapability), 40)
		gpu.DisplayActive = limitText(strings.TrimSpace(gpu.DisplayActive), 40)
		gpu.DisplayAttached = limitText(strings.TrimSpace(gpu.DisplayAttached), 40)
		gpu.PersistenceMode = limitText(strings.TrimSpace(gpu.PersistenceMode), 40)
		gpu.DriverModel = limitText(strings.TrimSpace(gpu.DriverModel), 80)
		gpu.ECCModeCurrent = limitText(strings.TrimSpace(gpu.ECCModeCurrent), 80)
		gpu.MIGModeCurrent = limitText(strings.TrimSpace(gpu.MIGModeCurrent), 80)
		gpu.ClockThrottleReasons = limitText(strings.TrimSpace(gpu.ClockThrottleReasons), 200)
		gpu.CollectionError = limitText(strings.TrimSpace(gpu.CollectionError), 240)
	}
	if len(report.CollectionErrors) > 20 {
		report.CollectionErrors = report.CollectionErrors[:20]
	}
	for index := range report.CollectionErrors {
		report.CollectionErrors[index] = limitText(strings.TrimSpace(report.CollectionErrors[index]), 240)
	}
	return report
}

func (a *App) authenticateAgent(w http.ResponseWriter, r *http.Request) (string, []byte, bool) {
	deviceID := r.Header.Get(auth.HeaderDeviceID)
	rateKey := clientIP(r) + ":" + deviceID
	if !a.agentRate.Allow(rateKey, time.Now()) {
		writeError(w, http.StatusTooManyRequests, "too many agent requests")
		return "", nil, false
	}
	if r.ContentLength > 2*1024*1024 {
		writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return "", nil, false
	}
	body, err := readBody(r, 2*1024*1024)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return "", nil, false
	}
	deviceAuth, exists := a.meta.DeviceAuth(deviceID)
	if !exists {
		_ = a.meta.AddAudit("device_auth_failed", "unknown device attempted authentication")
		writeError(w, http.StatusUnauthorized, "unknown device")
		return "", nil, false
	}
	if !deviceAuth.Enabled {
		_ = a.meta.AddAudit("device_auth_failed", fmt.Sprintf("disabled device %s attempted authentication", deviceID))
		writeError(w, http.StatusForbidden, "device disabled")
		return "", nil, false
	}
	nonce := r.Header.Get(auth.HeaderNonce)
	if nonce == "" {
		writeError(w, http.StatusUnauthorized, "missing nonce")
		return "", nil, false
	}
	if err := auth.Verify(
		r.Method,
		r.URL.EscapedPath(),
		body,
		deviceID,
		r.Header.Get(auth.HeaderTimestamp),
		nonce,
		r.Header.Get(auth.HeaderSignature),
		deviceAuth.Secret,
		time.Now().UTC(),
		5*time.Minute,
	); err != nil {
		legacyAllowed := a.meta.ServiceConfig().LegacyAgentAuth && allowLegacyAgentSignature(deviceAuth.AgentVersion)
		if !legacyAllowed || auth.VerifyLegacy(
			r.Method,
			r.URL.EscapedPath(),
			body,
			r.Header.Get(auth.HeaderTimestamp),
			nonce,
			r.Header.Get(auth.HeaderSignature),
			deviceAuth.Secret,
			time.Now().UTC(),
			5*time.Minute,
		) != nil {
			_ = a.meta.AddAudit("device_auth_failed", fmt.Sprintf("authentication failed for %s: %v", deviceID, err))
			writeError(w, http.StatusUnauthorized, err.Error())
			return "", nil, false
		}
		a.auditLegacyAgentSignature(deviceID, deviceAuth.AgentVersion)
	}
	if !a.nonces.Accept(deviceID, nonce, time.Now()) {
		writeError(w, http.StatusConflict, "replayed nonce")
		return "", nil, false
	}
	return deviceID, body, true
}

func allowLegacyAgentSignature(agentVersion string) bool {
	parts, ok := parseVersionParts(agentVersion)
	if !ok {
		return false
	}
	target := [3]int{0, 1, 9}
	for index := 0; index < len(parts); index++ {
		if parts[index] != target[index] {
			return parts[index] < target[index]
		}
	}
	return false
}

func parseVersionParts(raw string) ([3]int, bool) {
	var out [3]int
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "v"))
	if raw == "" {
		return out, false
	}
	parts := strings.Split(raw, ".")
	if len(parts) < 3 {
		return out, false
	}
	for index := 0; index < 3; index++ {
		part := parts[index]
		digits := strings.Builder{}
		for _, item := range part {
			if item < '0' || item > '9' {
				break
			}
			digits.WriteRune(item)
		}
		if digits.Len() == 0 {
			return out, false
		}
		value, err := strconv.Atoi(digits.String())
		if err != nil {
			return out, false
		}
		out[index] = value
	}
	return out, true
}

func (a *App) auditLegacyAgentSignature(deviceID, agentVersion string) {
	now := time.Now().UTC()
	a.agentAuthCompatMu.Lock()
	last := a.legacyAgentAuthAudit[deviceID]
	if !last.IsZero() && now.Sub(last) < time.Hour {
		a.agentAuthCompatMu.Unlock()
		return
	}
	a.legacyAgentAuthAudit[deviceID] = now
	a.agentAuthCompatMu.Unlock()
	_ = a.meta.AddAuditEvent(AuditEvent{
		At:       now,
		Type:     "device_auth_legacy_signature",
		Message:  fmt.Sprintf("accepted legacy HMAC signature for %s from agent %s; upgrade the agent to %s or newer", deviceID, agentVersion, version.Version),
		DeviceID: deviceID,
	})
}

func (a *App) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.sessions.Valid(r) {
			writeError(w, http.StatusUnauthorized, "login required")
			return
		}
		if unsafeBrowserMethod(r.Method) && !a.validSameOriginRequest(r) {
			_ = a.addAuditForRequest(r, "csrf_rejected", "rejected management request with invalid origin", "")
			writeError(w, http.StatusForbidden, "invalid request origin")
			return
		}
		next(w, r)
	}
}

func unsafeBrowserMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func (a *App) validSameOriginRequest(r *http.Request) bool {
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		return a.sameOrigin(r, origin)
	}
	if referer := strings.TrimSpace(r.Header.Get("Referer")); referer != "" {
		return a.sameOrigin(r, referer)
	}
	return false
}

func (a *App) sameOrigin(r *http.Request, raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Scheme, requestScheme(r, a.scheme)) && sameHost(parsed.Host, r.Host)
}

func requestScheme(r *http.Request, fallback string) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		if comma := strings.IndexByte(forwarded, ','); comma >= 0 {
			forwarded = strings.TrimSpace(forwarded[:comma])
		}
		if forwarded == "http" || forwarded == "https" {
			return forwarded
		}
	}
	if r.TLS != nil {
		return "https"
	}
	if fallback != "" {
		return fallback
	}
	return "http"
}

func sameHost(left, right string) bool {
	leftHost, leftPort, leftErr := net.SplitHostPort(left)
	rightHost, rightPort, rightErr := net.SplitHostPort(right)
	if leftErr == nil && rightErr == nil {
		return strings.EqualFold(leftHost, rightHost) && leftPort == rightPort
	}
	return strings.EqualFold(strings.TrimSuffix(left, "."), strings.TrimSuffix(right, "."))
}

func (a *App) addAuditForRequest(r *http.Request, eventType, message, deviceID string) error {
	requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if requestID == "" {
		if generated, err := randomHex(8); err == nil {
			requestID = generated
		}
	}
	return a.meta.AddAuditEvent(AuditEvent{
		Type:      eventType,
		Message:   message,
		Actor:     "admin",
		RemoteIP:  clientIP(r),
		DeviceID:  deviceID,
		RequestID: requestID,
	})
}

func readBody(r *http.Request, limit int64) ([]byte, error) {
	defer r.Body.Close()
	var reader io.Reader = io.LimitReader(r.Body, limit+1)
	if strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = io.LimitReader(gz, limit+1)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("request body too large")
	}
	return body, nil
}

func decodeJSON(r *http.Request, out any, limit int64) error {
	body, err := readBody(r, limit)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, model.APIError{Error: message})
}

func writeRetryAfterError(w http.ResponseWriter, status int, message string, retryAfter time.Duration) {
	seconds := retryAfterSeconds(retryAfter)
	if seconds > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
	}
	writeJSON(w, status, model.APIError{Error: message, RetryAfterSeconds: seconds})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", strictCSPHeader)
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func parseHours(r *http.Request, fallback int) int {
	hours := fallback
	if raw := r.URL.Query().Get("hours"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 && parsed <= 24*30 {
			hours = parsed
		}
	}
	return hours
}

func (a *App) deviceIDSet() map[string]bool {
	devices := a.meta.ListDevices()
	out := make(map[string]bool, len(devices))
	for _, device := range devices {
		out[device.ID] = true
	}
	return out
}

func filterProcessesByDevices(items []StoredProcessSnapshot, deviceIDs map[string]bool) []StoredProcessSnapshot {
	out := make([]StoredProcessSnapshot, 0, len(items))
	for _, item := range items {
		if deviceIDs[item.DeviceID] {
			out = append(out, item)
		}
	}
	return out
}

func filterStatsByDevices(items []GPUStats, deviceIDs map[string]bool) []GPUStats {
	out := make([]GPUStats, 0, len(items))
	for _, item := range items {
		if deviceIDs[item.DeviceID] {
			out = append(out, item)
		}
	}
	return out
}

func (a *App) setupStatus(r *http.Request) setupStatusResponse {
	return setupStatusResponse{
		SetupRequired: !a.meta.HasAdminPassword(),
		SetupComplete: a.meta.SetupComplete(),
		Service:       a.serviceStatus(r),
	}
}

func (a *App) serviceStatus(r *http.Request) serviceStatus {
	return a.serviceStatusFromConfig(a.meta.ServiceConfig(), r)
}

func (a *App) guestServiceStatus() guestServiceStatus {
	config := a.meta.ServiceConfig()
	return guestServiceStatus{
		CurrentScheme: a.Scheme(),
		Language:      config.Language,
		GuestEnabled:  config.GuestEnabled,
	}
}

func serviceStatusFromGuest(status guestServiceStatus) serviceStatus {
	return serviceStatus{
		CurrentScheme: status.CurrentScheme,
		Language:      status.Language,
		GuestEnabled:  status.GuestEnabled,
	}
}

func (a *App) serviceStatusFromConfig(config ServiceConfig, r *http.Request) serviceStatus {
	currentPort, _ := portFromAddr(a.config.Addr)
	if config.Port == 0 {
		config.Port = currentPort
	}
	if config.Addr == "" {
		config.Addr = a.config.Addr
	}
	if config.MinFreeBytes == 0 {
		config.MinFreeBytes = a.config.MinFreeBytes
	}
	status := serviceStatus{
		CurrentAddr:       a.config.Addr,
		CurrentScheme:     a.Scheme(),
		ConfiguredAddr:    config.Addr,
		ConfiguredPort:    config.Port,
		HTTPSEnabled:      config.HTTPS,
		Language:          config.Language,
		GuestEnabled:      config.GuestEnabled,
		UpdateProxy:       config.UpdateProxy,
		AutoUpdateEnabled: config.AutoUpdateOn(),
		LegacyAgentAuth:   config.LegacyAgentAuth,
		AgentUpdate:       config.AgentUpdate,
		MinFreeBytes:      config.MinFreeBytes,
		Energy:            config.EnergySettings(),
		CertNotAfter:      config.CertNotAfter,
		ConfigRevision:    config.ConfigRevision,
		UpdatedAt:         config.UpdatedAt,
		RestartRequired:   a.restartRequired(config),
		FirstStartupHTTP:  a.Scheme() == "http" && !a.meta.SetupComplete(),
		ManagementBaseURL: "",
	}
	host := "127.0.0.1"
	if r != nil && r.Host != "" {
		host = r.Host
		if rawHost, _, err := splitHostPort(r.Host); err == nil && rawHost != "" {
			host = rawHost
		}
	}
	if config.Port > 0 {
		status.ManagementBaseURL = fmt.Sprintf("%s://%s:%d", a.Scheme(), host, config.Port)
	}
	return status
}

func (a *App) restartRequired(config ServiceConfig) bool {
	currentPort, currentPortOK := portFromAddr(a.config.Addr)
	portChanged := currentPortOK && config.Port > 0 && config.Port != currentPort
	schemeChanged := config.HTTPS != (a.Scheme() == "https")
	return portChanged || schemeChanged
}

type diagnosticsReport struct {
	Product       string                   `json:"product"`
	Version       string                   `json:"version"`
	Commit        string                   `json:"commit"`
	BuildTime     string                   `json:"build_time,omitempty"`
	GeneratedAt   time.Time                `json:"generated_at"`
	UptimeSeconds int64                    `json:"uptime_seconds"`
	Runtime       diagnosticsRuntime       `json:"runtime"`
	Service       serviceStatus            `json:"service"`
	Telemetry     diagnosticsTelemetry     `json:"telemetry"`
	Storage       diagnosticsStorage       `json:"storage"`
	Devices       []diagnosticsDevice      `json:"devices"`
	GPUs          []diagnosticsGPU         `json:"gpus"`
	Processes     diagnosticsProcesses     `json:"processes"`
	Update        *diagnosticsUpdateStatus `json:"update,omitempty"`
	Audit         []diagnosticsAuditEvent  `json:"audit"`
}

type diagnosticsRuntime struct {
	GoVersion        string `json:"go_version"`
	GOOS             string `json:"goos"`
	GOARCH           string `json:"goarch"`
	NumCPU           int    `json:"num_cpu"`
	NumGoroutine     int    `json:"num_goroutine"`
	MemoryAllocBytes uint64 `json:"memory_alloc_bytes"`
	MemorySysBytes   uint64 `json:"memory_sys_bytes"`
	HeapAllocBytes   uint64 `json:"heap_alloc_bytes"`
	HeapObjects      uint64 `json:"heap_objects"`
	NextGCBytes      uint64 `json:"next_gc_bytes"`
	NumGC            uint32 `json:"num_gc"`
}

type diagnosticsTelemetry struct {
	Enabled         bool      `json:"enabled"`
	Endpoint        string    `json:"endpoint,omitempty"`
	ProxyConfigured bool      `json:"proxy_configured"`
	LastReportAt    time.Time `json:"last_report_at,omitempty"`
	LastSuccessAt   time.Time `json:"last_success_at,omitempty"`
	NextReportAfter time.Time `json:"next_report_after,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
}

type diagnosticsStorage struct {
	DataDir             string     `json:"data_dir"`
	DatabaseSizeBytes   uint64     `json:"database_size_bytes"`
	RetentionHours      int        `json:"retention_hours"`
	MetricStoredDays    int        `json:"metric_stored_days"`
	MinFreeSpaceBytes   uint64     `json:"min_free_space_bytes"`
	Disk                DiskStatus `json:"disk"`
	DiskError           string     `json:"disk_error,omitempty"`
	MetricSegmentCount  int        `json:"metric_segment_count"`
	MetricSegmentOldest time.Time  `json:"metric_segment_oldest,omitempty"`
	MetricSegmentNewest time.Time  `json:"metric_segment_newest,omitempty"`
	MetricSegmentError  string     `json:"metric_segment_error,omitempty"`
}

type diagnosticsMetricSegments struct {
	Count  int
	Oldest time.Time
	Newest time.Time
	Error  string
}

type diagnosticsDevice struct {
	ID           string    `json:"id"`
	Alias        string    `json:"alias"`
	Enabled      bool      `json:"enabled"`
	Status       string    `json:"status"`
	Hostname     string    `json:"hostname,omitempty"`
	OS           string    `json:"os,omitempty"`
	OSVersion    string    `json:"os_version,omitempty"`
	AgentVersion string    `json:"agent_version,omitempty"`
	GPUCount     int       `json:"gpu_count"`
	CreatedAt    time.Time `json:"created_at"`
	LastSeenAt   time.Time `json:"last_seen_at,omitempty"`
	LastSampleAt time.Time `json:"last_sample_at,omitempty"`
	LastError    string    `json:"last_error,omitempty"`
}

type diagnosticsGPU struct {
	DeviceID              string    `json:"device_id"`
	GPUID                 string    `json:"gpu_id"`
	Timestamp             time.Time `json:"timestamp"`
	Name                  string    `json:"name"`
	MemoryTotalBytes      uint64    `json:"memory_total_bytes"`
	MemoryUsedBytes       uint64    `json:"memory_used_bytes"`
	MemoryFreeBytes       uint64    `json:"memory_free_bytes,omitempty"`
	MemoryReservedBytes   uint64    `json:"memory_reserved_bytes,omitempty"`
	UtilizationGPUPercent *float64  `json:"utilization_gpu_percent,omitempty"`
	UtilizationMemPercent *float64  `json:"utilization_memory_percent,omitempty"`
	TemperatureCelsius    *float64  `json:"temperature_celsius,omitempty"`
	TemperatureMemCelsius *float64  `json:"temperature_memory_celsius,omitempty"`
	PowerDrawWatts        *float64  `json:"power_draw_watts,omitempty"`
	PowerLimitWatts       *float64  `json:"power_limit_watts,omitempty"`
	FanSpeedPercent       *float64  `json:"fan_speed_percent,omitempty"`
	GraphicsClockMHz      *float64  `json:"graphics_clock_mhz,omitempty"`
	MemoryClockMHz        *float64  `json:"memory_clock_mhz,omitempty"`
	SMClockMHz            *float64  `json:"sm_clock_mhz,omitempty"`
	PState                string    `json:"pstate,omitempty"`
	PCIeLinkGeneration    string    `json:"pcie_link_generation,omitempty"`
	PCIeLinkWidth         string    `json:"pcie_link_width,omitempty"`
	PCIeLinkGenerationMax string    `json:"pcie_link_generation_max,omitempty"`
	PCIeLinkWidthMax      string    `json:"pcie_link_width_max,omitempty"`
	ClockThrottleReasons  string    `json:"clock_throttle_reasons,omitempty"`
	CollectionError       string    `json:"collection_error,omitempty"`
}

type diagnosticsProcesses struct {
	TotalProcessCount int                     `json:"total_process_count"`
	ByGPU             []diagnosticsProcessGPU `json:"by_gpu"`
}

type diagnosticsProcessGPU struct {
	DeviceID     string    `json:"device_id"`
	GPUID        string    `json:"gpu_id"`
	ProcessCount int       `json:"process_count"`
	LastSeenAt   time.Time `json:"last_seen_at,omitempty"`
}

type diagnosticsUpdateStatus struct {
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
}

type diagnosticsAuditEvent struct {
	At        time.Time `json:"at"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Actor     string    `json:"actor,omitempty"`
	RemoteIP  string    `json:"remote_ip,omitempty"`
	DeviceID  string    `json:"device_id,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
}

type overviewResponse struct {
	ServerTime         time.Time               `json:"server_time"`
	DeviceCount        int                     `json:"device_count"`
	OnlineDeviceCount  int                     `json:"online_device_count"`
	GPUCount           int                     `json:"gpu_count"`
	AverageUtilization float64                 `json:"average_utilization"`
	MemoryUsedBytes    uint64                  `json:"memory_used_bytes"`
	MemoryTotalBytes   uint64                  `json:"memory_total_bytes"`
	PowerDrawWatts     float64                 `json:"power_draw_watts"`
	HotGPUCount        int                     `json:"hot_gpu_count"`
	Disk               DiskStatus              `json:"disk"`
	Devices            []deviceView            `json:"devices"`
	LatestGPUs         []StoredGPU             `json:"latest_gpus"`
	LatestProcesses    []StoredProcessSnapshot `json:"latest_processes"`
	RetentionHours     int                     `json:"retention_hours"`
	MetricStoredDays   int                     `json:"metric_stored_days"`
	MinFreeSpaceBytes  uint64                  `json:"min_free_space_bytes"`
	DatabaseSizeBytes  uint64                  `json:"database_size_bytes"`
	SetupComplete      bool                    `json:"setup_complete"`
	Service            serviceStatus           `json:"service"`
	Guest              bool                    `json:"guest,omitempty"`
}

type setupStatusResponse struct {
	SetupRequired bool          `json:"setup_required"`
	SetupComplete bool          `json:"setup_complete"`
	Service       serviceStatus `json:"service"`
}

type serviceStatus struct {
	CurrentAddr       string                  `json:"current_addr"`
	CurrentScheme     string                  `json:"current_scheme"`
	ConfiguredAddr    string                  `json:"configured_addr"`
	ConfiguredPort    int                     `json:"configured_port"`
	HTTPSEnabled      bool                    `json:"https_enabled"`
	Language          string                  `json:"language"`
	GuestEnabled      bool                    `json:"guest_enabled"`
	UpdateProxy       string                  `json:"update_proxy,omitempty"`
	AutoUpdateEnabled bool                    `json:"auto_update_enabled"`
	LegacyAgentAuth   bool                    `json:"legacy_agent_auth_enabled"`
	AgentUpdate       model.AgentUpdatePolicy `json:"agent_update"`
	MinFreeBytes      uint64                  `json:"min_free_bytes"`
	Energy            EnergySettings          `json:"energy"`
	CertNotAfter      time.Time               `json:"cert_not_after,omitempty"`
	ConfigRevision    int                     `json:"config_revision"`
	UpdatedAt         time.Time               `json:"updated_at,omitempty"`
	RestartRequired   bool                    `json:"restart_required"`
	FirstStartupHTTP  bool                    `json:"first_startup_http"`
	ManagementBaseURL string                  `json:"management_base_url,omitempty"`
}

type guestServiceStatus struct {
	CurrentScheme string `json:"current_scheme"`
	Language      string `json:"language"`
	GuestEnabled  bool   `json:"guest_enabled"`
}

type deviceView struct {
	ID           string    `json:"id"`
	Alias        string    `json:"alias"`
	Enabled      bool      `json:"enabled"`
	Status       string    `json:"status,omitempty"`
	Hostname     string    `json:"hostname,omitempty"`
	OS           string    `json:"os,omitempty"`
	OSVersion    string    `json:"os_version,omitempty"`
	AgentVersion string    `json:"agent_version,omitempty"`
	GPUCount     int       `json:"gpu_count"`
	LastSeenAt   time.Time `json:"last_seen_at,omitempty"`
	LastSampleAt time.Time `json:"last_sample_at,omitempty"`
	LastError    string    `json:"last_error,omitempty"`
}

func toDeviceView(device Device) deviceView {
	return deviceView{
		ID:           device.ID,
		Alias:        device.Alias,
		Enabled:      device.Enabled,
		Hostname:     device.Hostname,
		OS:           device.OS,
		OSVersion:    device.OSVersion,
		AgentVersion: device.AgentVersion,
		GPUCount:     device.GPUCount,
		LastSeenAt:   device.LastSeenAt,
		LastSampleAt: device.LastSampleAt,
		LastError:    device.LastError,
	}
}

func HumanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
