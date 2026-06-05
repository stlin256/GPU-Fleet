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
	"os"
	"path/filepath"
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
	Addr              string
	AddrExplicit      bool
	DataDir           string
	MinFreeBytes      uint64
	Retention         time.Duration
	BootstrapDeviceID string
	BootstrapSecret   string
	AdminPassword     string
	WebDir            string
	RepoDir           string
}

type App struct {
	config     Config
	meta       *MetadataStore
	metrics    *MetricsStore
	processes  *ProcessStore
	nonces     *NonceStore
	sessions   *SessionStore
	loginRate  *RateLimiter
	loginGuard *LoginGuard
	agentRate  *RateLimiter
	updateMu   sync.Mutex
	logger     *log.Logger
	scheme     string
}

const webSessionTTL = 30 * 24 * time.Hour

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

	return &App{
		config:     config,
		meta:       meta,
		metrics:    metrics,
		processes:  processes,
		nonces:     NewNonceStore(10 * time.Minute),
		sessions:   NewSessionStore(webSessionTTL, meta),
		loginRate:  NewRateLimiter(10, time.Minute),
		loginGuard: NewLoginGuard(5, 30*time.Minute, 5*time.Minute, time.Hour),
		agentRate:  NewRateLimiter(240, time.Minute),
		logger:     logger,
		scheme:     scheme,
	}, generatedPassword, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleIndex)
	mux.HandleFunc("/api/v1/setup/status", a.handleSetupStatus)
	mux.HandleFunc("/api/v1/setup/apply", a.handleSetupApply)
	mux.HandleFunc("/api/v1/auth/login", a.handleLogin)
	mux.HandleFunc("/api/v1/auth/logout", a.handleLogout)
	mux.HandleFunc("/api/v1/version", a.requireSession(a.handleVersion))
	mux.HandleFunc("/api/v1/overview", a.requireSession(a.handleOverview))
	mux.HandleFunc("/api/v1/devices", a.requireSession(a.handleDevices))
	mux.HandleFunc("/api/v1/gpus/", a.requireSession(a.handleGPUSeries))
	mux.HandleFunc("/api/v1/stats/gpu-utilization", a.requireSession(a.handleGPUStats))
	mux.HandleFunc("/api/v1/processes/latest", a.requireSession(a.handleLatestProcesses))
	mux.HandleFunc("/api/v1/admin/setup/reopen", a.requireSession(a.handleSetupReopen))
	mux.HandleFunc("/api/v1/admin/setup/apply", a.requireSession(a.handleSetupApplyAuthenticated))
	mux.HandleFunc("/api/v1/admin/password", a.requireSession(a.handleAdminPassword))
	mux.HandleFunc("/api/v1/admin/server-config", a.requireSession(a.handleAdminServerConfig))
	mux.HandleFunc("/api/v1/admin/certificate", a.requireSession(a.handleAdminCertificate))
	mux.HandleFunc("/api/v1/admin/database/download", a.requireSession(a.handleDatabaseDownload))
	mux.HandleFunc("/api/v1/admin/update/status", a.requireSession(a.handleUpdateStatus))
	mux.HandleFunc("/api/v1/admin/update/apply", a.requireSession(a.handleUpdateApply))
	mux.HandleFunc("/api/v1/admin/devices", a.requireSession(a.handleCreateDevice))
	mux.HandleFunc("/api/v1/admin/devices/", a.requireSession(a.handleAdminDeviceAction))
	mux.HandleFunc("/api/v1/agent/heartbeat", a.handleAgentHeartbeat)
	mux.HandleFunc("/api/v1/agent/samples", a.handleAgentSamples)
	mux.HandleFunc("/api/v1/agent/process-snapshots", a.handleAgentProcesses)
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
	if err := a.sessions.Create(w, r.TLS != nil); err != nil {
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
	writeJSON(w, http.StatusOK, version.Current())
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
		Port int `json:"port"`
	}
	if err := decodeJSON(r, &body, 1<<20); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	config, err := a.meta.UpdateServicePort(body.Port)
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
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"service":          a.serviceStatusFromConfig(config, r),
		"restart_required": a.restartRequired(config),
	})
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
		if relSlash != "metadata.json" && relSlash != "processes.json" && !strings.HasPrefix(relSlash, "metrics/") {
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

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.sessions.Clear(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleOverview(w http.ResponseWriter, r *http.Request) {
	devices := a.meta.ListDevices()
	diskStatus, _ := a.metrics.DiskStatus()

	deviceViews := make([]deviceView, 0, len(devices))
	deviceIDs := make(map[string]bool, len(devices))
	now := time.Now().UTC()
	online := 0
	for _, device := range devices {
		deviceIDs[device.ID] = true
		status := "offline"
		if device.Enabled && !device.LastSeenAt.IsZero() && now.Sub(device.LastSeenAt) <= 45*time.Second {
			status = "online"
			online++
		}
		deviceViews = append(deviceViews, deviceView{
			ID:           device.ID,
			Alias:        device.Alias,
			Enabled:      device.Enabled,
			Status:       status,
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
	sort.Slice(deviceViews, func(i, j int) bool { return deviceViews[i].Alias < deviceViews[j].Alias })

	latest := a.metrics.Latest()
	filteredLatest := make([]StoredGPU, 0, len(latest))
	totalUtil := 0.0
	utilCount := 0
	usedMem := uint64(0)
	totalMem := uint64(0)
	hot := 0
	for _, item := range latest {
		if !deviceIDs[item.DeviceID] {
			continue
		}
		filteredLatest = append(filteredLatest, item)
		if item.GPU.UtilizationGPUPercent != nil {
			totalUtil += *item.GPU.UtilizationGPUPercent
			utilCount++
		}
		usedMem += item.GPU.MemoryUsedBytes
		totalMem += item.GPU.MemoryTotalBytes
		if item.GPU.TemperatureCelsius != nil && *item.GPU.TemperatureCelsius >= 85 {
			hot++
		}
	}
	avgUtil := 0.0
	if utilCount > 0 {
		avgUtil = totalUtil / float64(utilCount)
	}

	writeJSON(w, http.StatusOK, overviewResponse{
		ServerTime:         now,
		DeviceCount:        len(devices),
		OnlineDeviceCount:  online,
		GPUCount:           len(filteredLatest),
		AverageUtilization: avgUtil,
		MemoryUsedBytes:    usedMem,
		MemoryTotalBytes:   totalMem,
		HotGPUCount:        hot,
		Disk:               diskStatus,
		Devices:            deviceViews,
		LatestGPUs:         filteredLatest,
		LatestProcesses:    filterProcessesByDevices(a.processes.Latest("", ""), deviceIDs),
		RetentionHours:     int(a.config.Retention.Hours()),
		MinFreeSpaceBytes:  a.config.MinFreeBytes,
		SetupComplete:      a.meta.SetupComplete(),
		Service:            a.serviceStatus(r),
	})
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
		writeJSON(w, http.StatusOK, map[string]any{"device": toDeviceView(device)})
	case "disable":
		device, err := a.meta.SetDeviceEnabled(deviceID, false)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"device": toDeviceView(device)})
	case "rotate-secret":
		device, secret, err := a.meta.RotateDeviceSecret(deviceID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
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
	points, err := a.metrics.Series(deviceID, gpuID, time.Now().Add(-time.Duration(hours)*time.Hour))
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
	secret, enabled, exists := a.meta.DeviceSecret(deviceID)
	if !exists {
		_ = a.meta.AddAudit("device_auth_failed", "unknown device attempted authentication")
		writeError(w, http.StatusUnauthorized, "unknown device")
		return "", nil, false
	}
	if !enabled {
		_ = a.meta.AddAudit("device_auth_failed", fmt.Sprintf("disabled device %s attempted authentication", deviceID))
		writeError(w, http.StatusForbidden, "device disabled")
		return "", nil, false
	}
	nonce := r.Header.Get(auth.HeaderNonce)
	if nonce == "" {
		writeError(w, http.StatusUnauthorized, "missing nonce")
		return "", nil, false
	}
	if !a.nonces.Accept(deviceID, nonce, time.Now()) {
		writeError(w, http.StatusConflict, "replayed nonce")
		return "", nil, false
	}
	if err := auth.Verify(
		r.Method,
		r.URL.EscapedPath(),
		body,
		r.Header.Get(auth.HeaderTimestamp),
		nonce,
		r.Header.Get(auth.HeaderSignature),
		secret,
		time.Now().UTC(),
		5*time.Minute,
	); err != nil {
		_ = a.meta.AddAudit("device_auth_failed", fmt.Sprintf("authentication failed for %s: %v", deviceID, err))
		writeError(w, http.StatusUnauthorized, err.Error())
		return "", nil, false
	}
	return deviceID, body, true
}

func (a *App) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.sessions.Valid(r) {
			writeError(w, http.StatusUnauthorized, "login required")
			return
		}
		next(w, r)
	}
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
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
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

func (a *App) serviceStatusFromConfig(config ServiceConfig, r *http.Request) serviceStatus {
	currentPort, _ := portFromAddr(a.config.Addr)
	if config.Port == 0 {
		config.Port = currentPort
	}
	if config.Addr == "" {
		config.Addr = a.config.Addr
	}
	status := serviceStatus{
		CurrentAddr:       a.config.Addr,
		CurrentScheme:     a.Scheme(),
		ConfiguredAddr:    config.Addr,
		ConfiguredPort:    config.Port,
		HTTPSEnabled:      config.HTTPS,
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

type overviewResponse struct {
	ServerTime         time.Time               `json:"server_time"`
	DeviceCount        int                     `json:"device_count"`
	OnlineDeviceCount  int                     `json:"online_device_count"`
	GPUCount           int                     `json:"gpu_count"`
	AverageUtilization float64                 `json:"average_utilization"`
	MemoryUsedBytes    uint64                  `json:"memory_used_bytes"`
	MemoryTotalBytes   uint64                  `json:"memory_total_bytes"`
	HotGPUCount        int                     `json:"hot_gpu_count"`
	Disk               DiskStatus              `json:"disk"`
	Devices            []deviceView            `json:"devices"`
	LatestGPUs         []StoredGPU             `json:"latest_gpus"`
	LatestProcesses    []StoredProcessSnapshot `json:"latest_processes"`
	RetentionHours     int                     `json:"retention_hours"`
	MinFreeSpaceBytes  uint64                  `json:"min_free_space_bytes"`
	SetupComplete      bool                    `json:"setup_complete"`
	Service            serviceStatus           `json:"service"`
}

type setupStatusResponse struct {
	SetupRequired bool          `json:"setup_required"`
	SetupComplete bool          `json:"setup_complete"`
	Service       serviceStatus `json:"service"`
}

type serviceStatus struct {
	CurrentAddr       string    `json:"current_addr"`
	CurrentScheme     string    `json:"current_scheme"`
	ConfiguredAddr    string    `json:"configured_addr"`
	ConfiguredPort    int       `json:"configured_port"`
	HTTPSEnabled      bool      `json:"https_enabled"`
	CertNotAfter      time.Time `json:"cert_not_after,omitempty"`
	ConfigRevision    int       `json:"config_revision"`
	UpdatedAt         time.Time `json:"updated_at,omitempty"`
	RestartRequired   bool      `json:"restart_required"`
	FirstStartupHTTP  bool      `json:"first_startup_http"`
	ManagementBaseURL string    `json:"management_base_url,omitempty"`
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
