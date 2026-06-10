package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"gpufleet/internal/version"
)

const (
	defaultTelemetryEndpoint = "https://gpufleet-telemetry.stlin256.workers.dev/v1/report"
	telemetryDefaultInterval = 24 * time.Hour
	telemetryStartupDelay    = 2 * time.Minute
	telemetryCheckInterval   = time.Hour
	telemetryRetryInterval   = 6 * time.Hour
	telemetryActiveWindow    = 7 * 24 * time.Hour
)

type telemetryReport struct {
	SchemaVersion  int       `json:"schema_version"`
	InstallIDHash  string    `json:"install_id_hash"`
	Version        string    `json:"version"`
	Commit         string    `json:"commit,omitempty"`
	ServerOS       string    `json:"server_os"`
	ServerArch     string    `json:"server_arch"`
	ClientsTotal   int       `json:"clients_total"`
	ClientsActive7 int       `json:"clients_active_7d"`
	GPUsTotal      int       `json:"gpus_total"`
	GPUsActive7    int       `json:"gpus_active_7d"`
	ReportedAt     time.Time `json:"reported_at"`
}

func (a *App) startTelemetryLoop() {
	go func() {
		time.Sleep(telemetryStartupDelay)
		ticker := time.NewTicker(telemetryCheckInterval)
		defer ticker.Stop()
		for {
			a.runTelemetryIfDue(time.Now().UTC())
			<-ticker.C
		}
	}()
}

func (a *App) runTelemetryIfDue(now time.Time) bool {
	state := a.meta.TelemetryState()
	if !state.NextReportAfter.IsZero() && now.Before(state.NextReportAfter) {
		return false
	}
	report, err := a.buildTelemetryReport(now)
	if err != nil {
		a.recordTelemetryError(now, err, telemetryRetryInterval)
		return true
	}
	if err := a.submitTelemetryReport(report); err != nil {
		a.recordTelemetryError(now, err, telemetryRetryInterval)
		return true
	}
	next := now.Add(a.config.TelemetryInterval).Add(telemetryJitter(time.Hour))
	if err := a.meta.RecordTelemetrySuccess(now, next); err != nil {
		a.logger.Printf("anonymous telemetry state update failed: %v", err)
	}
	return true
}

func (a *App) recordTelemetryError(now time.Time, err error, retryAfter time.Duration) {
	next := now.Add(retryAfter).Add(telemetryJitter(30 * time.Minute))
	message := redactTelemetryError(err.Error(), a.meta.ServiceConfig().UpdateProxy)
	if saveErr := a.meta.RecordTelemetryError(now, next, message); saveErr != nil {
		a.logger.Printf("anonymous telemetry state update failed: %v", saveErr)
	}
	a.logger.Printf("anonymous telemetry report failed: %s", message)
}

func (a *App) buildTelemetryReport(now time.Time) (telemetryReport, error) {
	installID, err := a.meta.EnsureTelemetryInstallID()
	if err != nil {
		return telemetryReport{}, err
	}
	clientsTotal, clientsActive, gpusTotal, gpusActive := a.telemetryCounts(now)
	commit := strings.TrimSpace(version.Commit)
	if commit == "dev" {
		commit = ""
	}
	return telemetryReport{
		SchemaVersion:  1,
		InstallIDHash:  telemetryInstallHash(installID),
		Version:        version.Version,
		Commit:         commit,
		ServerOS:       runtime.GOOS,
		ServerArch:     runtime.GOARCH,
		ClientsTotal:   clientsTotal,
		ClientsActive7: clientsActive,
		GPUsTotal:      gpusTotal,
		GPUsActive7:    gpusActive,
		ReportedAt:     now.UTC(),
	}, nil
}

func (a *App) telemetryCounts(now time.Time) (clientsTotal, clientsActive, gpusTotal, gpusActive int) {
	devices := a.meta.ListDevices()
	latestCounts := map[string]int{}
	for _, item := range a.metrics.Latest() {
		latestCounts[item.DeviceID]++
	}
	activeSince := now.Add(-telemetryActiveWindow)
	for _, device := range devices {
		clientsTotal++
		gpuCount := device.GPUCount
		if gpuCount <= 0 {
			gpuCount = latestCounts[device.ID]
		}
		gpuCount = clampTelemetryCount(gpuCount, 8192)
		gpusTotal += gpuCount
		if device.Enabled && !device.LastSeenAt.IsZero() && !device.LastSeenAt.Before(activeSince) {
			clientsActive++
			gpusActive += gpuCount
		}
	}
	return clientsTotal, clientsActive, gpusTotal, gpusActive
}

func (a *App) submitTelemetryReport(report telemetryReport) error {
	raw, err := json.Marshal(report)
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, a.config.TelemetryEndpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "GPUFleet/"+version.Version)
	client, err := telemetryClient(a.meta.ServiceConfig().UpdateProxy)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("telemetry endpoint returned HTTP %d", response.StatusCode)
	}
	return nil
}

func telemetryClient(proxyURL string) (*http.Client, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{Timeout: 8 * time.Second}, nil
	}
	cloned := transport.Clone()
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL != "" {
		normalized, err := normalizeProxyURL(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("telemetry proxy URL invalid: %w", err)
		}
		parsed, err := url.Parse(normalized)
		if err != nil {
			return nil, fmt.Errorf("telemetry proxy URL invalid: %w", err)
		}
		cloned.Proxy = http.ProxyURL(parsed)
	}
	return &http.Client{Timeout: 8 * time.Second, Transport: cloned}, nil
}

func redactTelemetryError(message, proxyURL string) string {
	message = strings.TrimSpace(message)
	proxyURL = strings.TrimSpace(proxyURL)
	if message == "" || proxyURL == "" {
		return message
	}
	return strings.ReplaceAll(message, proxyURL, redactURLCredentials(proxyURL))
}

func telemetryInstallHash(installID string) string {
	sum := sha256.Sum256([]byte(installID))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func clampTelemetryCount(value, max int) int {
	if value < 0 {
		return 0
	}
	if value > max {
		return max
	}
	return value
}

func telemetryJitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	token, err := randomHex(8)
	if err != nil || len(token) < 16 {
		return 0
	}
	value, err := hex.DecodeString(token)
	if err != nil || len(value) < 8 {
		return 0
	}
	n := binary.BigEndian.Uint64(value)
	return time.Duration(n % uint64(max))
}
