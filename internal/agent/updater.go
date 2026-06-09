package agent

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gpufleet/internal/model"
	"gpufleet/internal/version"
)

var ErrAgentUpdateApplied = errors.New("agent update applied; restart required")

type UpdateManifest struct {
	Version         string           `json:"version"`
	CreatedAt       time.Time        `json:"created_at,omitempty"`
	MinAgentVersion string           `json:"min_agent_version,omitempty"`
	Artifacts       []UpdateArtifact `json:"artifacts"`
	Signature       string           `json:"signature"`
}

type UpdateArtifact struct {
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	URL         string `json:"url"`
	SHA256      string `json:"sha256"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
	Filename    string `json:"filename,omitempty"`
	Compression string `json:"compression,omitempty"`
}

type UpdateResult struct {
	Status         string
	TargetVersion  string
	ManifestURL    string
	ArtifactURL    string
	ArtifactSHA256 string
	Message        string
}

type AgentUpdater struct {
	CurrentVersion string
	OS             string
	Arch           string
	ExecutablePath string
	StagingDir     string
	HTTP           *http.Client
}

func (u AgentUpdater) ApplyPolicy(ctx context.Context, policy model.AgentUpdatePolicy) (UpdateResult, error) {
	result := UpdateResult{Status: "skipped", ManifestURL: policy.ManifestURL}
	if !policy.Enabled {
		result.Message = "policy disabled"
		return result, nil
	}
	manifest, err := u.FetchManifest(ctx, policy.ManifestURL, policy.PublicKey)
	if err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result, err
	}
	result.TargetVersion = strings.TrimPrefix(manifest.Version, "v")
	if !versionAllowed(u.currentVersion(), result.TargetVersion, policy) {
		result.Message = "target version not allowed by policy"
		return result, nil
	}
	artifact, ok := manifest.SelectArtifact(u.goos(), u.goarch())
	if !ok {
		err := fmt.Errorf("no artifact for %s/%s", u.goos(), u.goarch())
		result.Status = "failed"
		result.Message = err.Error()
		return result, err
	}
	result.ArtifactURL = artifact.URL
	result.ArtifactSHA256 = artifact.SHA256
	if policy.Mode == "notify" {
		result.Status = "available"
		result.Message = "update available"
		return result, nil
	}
	payload, err := u.DownloadArtifact(ctx, artifact)
	if err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result, err
	}
	if err := verifyArtifactSHA256(payload, artifact.SHA256); err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result, err
	}
	if err := u.Install(payload); err != nil {
		result.Status = "failed"
		result.Message = err.Error()
		return result, err
	}
	result.Status = "applied"
	result.Message = "artifact verified and installed"
	return result, nil
}

func (u AgentUpdater) FetchManifest(ctx context.Context, manifestURL, publicKey string) (UpdateManifest, error) {
	if strings.TrimSpace(manifestURL) == "" {
		return UpdateManifest{}, errors.New("manifest URL is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return UpdateManifest{}, err
	}
	res, err := u.httpClient().Do(req)
	if err != nil {
		return UpdateManifest{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return UpdateManifest{}, fmt.Errorf("manifest returned %s", res.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024))
	if err != nil {
		return UpdateManifest{}, err
	}
	var manifest UpdateManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return UpdateManifest{}, err
	}
	if err := manifest.Verify(publicKey); err != nil {
		return UpdateManifest{}, err
	}
	return manifest, nil
}

func (m UpdateManifest) Verify(publicKey string) error {
	keyRaw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(publicKey))
	if err != nil {
		return fmt.Errorf("decode update public key: %w", err)
	}
	if len(keyRaw) != ed25519.PublicKeySize {
		return fmt.Errorf("update public key must be %d bytes", ed25519.PublicKeySize)
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(m.Signature))
	if err != nil {
		return fmt.Errorf("decode manifest signature: %w", err)
	}
	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("manifest signature must be %d bytes", ed25519.SignatureSize)
	}
	canonical, err := m.SigningBytes()
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(keyRaw), canonical, signature) {
		return errors.New("manifest signature mismatch")
	}
	if strings.TrimSpace(m.Version) == "" {
		return errors.New("manifest version is required")
	}
	if len(m.Artifacts) == 0 {
		return errors.New("manifest must include at least one artifact")
	}
	return nil
}

func (m UpdateManifest) SigningBytes() ([]byte, error) {
	m.Signature = ""
	return json.Marshal(m)
}

func (m UpdateManifest) SelectArtifact(goos, goarch string) (UpdateArtifact, bool) {
	for _, artifact := range m.Artifacts {
		if strings.EqualFold(artifact.OS, goos) && strings.EqualFold(artifact.Arch, goarch) {
			return artifact, true
		}
	}
	return UpdateArtifact{}, false
}

func (u AgentUpdater) DownloadArtifact(ctx context.Context, artifact UpdateArtifact) ([]byte, error) {
	if strings.TrimSpace(artifact.URL) == "" {
		return nil, errors.New("artifact URL is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, artifact.URL, nil)
	if err != nil {
		return nil, err
	}
	res, err := u.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("artifact returned %s", res.Status)
	}
	limit := int64(256 * 1024 * 1024)
	if artifact.SizeBytes > 0 {
		limit = artifact.SizeBytes + 1
	}
	payload, err := io.ReadAll(io.LimitReader(res.Body, limit))
	if err != nil {
		return nil, err
	}
	if artifact.SizeBytes > 0 && int64(len(payload)) != artifact.SizeBytes {
		return nil, fmt.Errorf("artifact size mismatch: expected %d, got %d", artifact.SizeBytes, len(payload))
	}
	return payload, nil
}

func verifyArtifactSHA256(payload []byte, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return errors.New("artifact sha256 is required")
	}
	sum := sha256.Sum256(payload)
	actual := hex.EncodeToString(sum[:])
	if actual != expected {
		return fmt.Errorf("artifact sha256 mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

func (u AgentUpdater) Install(payload []byte) error {
	exe := u.executablePath()
	if exe == "" {
		return errors.New("agent executable path is unknown")
	}
	dir := u.StagingDir
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Dir(exe)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	next := filepath.Join(dir, filepath.Base(exe)+".next")
	backup := exe + ".bak"
	if err := os.WriteFile(next, payload, 0755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" && samePath(exe, currentExecutablePath()) {
		return scheduleWindowsReplacement(dir, exe, next, backup)
	}
	if err := os.Remove(backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := os.Stat(exe); err == nil {
		if err := os.Rename(exe, backup); err != nil {
			return fmt.Errorf("backup current agent binary: %w", err)
		}
	}
	if err := os.Rename(next, exe); err != nil {
		if _, restoreErr := os.Stat(backup); restoreErr == nil {
			_ = os.Rename(backup, exe)
		}
		return fmt.Errorf("replace agent binary: %w", err)
	}
	return nil
}

func scheduleWindowsReplacement(dir, exe, next, backup string) error {
	script := filepath.Join(dir, "gpufleet-agent-self-update.ps1")
	body := `$ErrorActionPreference = "Stop"
$pidToWait = [int]$args[0]
$exe = $args[1]
$next = $args[2]
$backup = $args[3]
Wait-Process -Id $pidToWait -Timeout 120 -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 500
if (Test-Path -LiteralPath $backup) { Remove-Item -LiteralPath $backup -Force }
if (Test-Path -LiteralPath $exe) { Move-Item -LiteralPath $exe -Destination $backup -Force }
Move-Item -LiteralPath $next -Destination $exe -Force
`
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		return err
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass", "-File", script, strconv.Itoa(os.Getpid()), exe, next, backup)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Windows self-update helper: %w", err)
	}
	return nil
}

func currentExecutablePath() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	return path
}

func samePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(leftAbs, rightAbs)
	}
	return leftAbs == rightAbs
}

func (u AgentUpdater) httpClient() *http.Client {
	if u.HTTP != nil {
		return u.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (u AgentUpdater) currentVersion() string {
	if strings.TrimSpace(u.CurrentVersion) != "" {
		return strings.TrimPrefix(strings.TrimSpace(u.CurrentVersion), "v")
	}
	return version.Version
}

func (u AgentUpdater) goos() string {
	if u.OS != "" {
		return u.OS
	}
	return runtime.GOOS
}

func (u AgentUpdater) goarch() string {
	if u.Arch != "" {
		return u.Arch
	}
	return runtime.GOARCH
}

func (u AgentUpdater) executablePath() string {
	if strings.TrimSpace(u.ExecutablePath) != "" {
		return u.ExecutablePath
	}
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	return path
}

func versionAllowed(current, target string, policy model.AgentUpdatePolicy) bool {
	target = strings.TrimPrefix(strings.TrimSpace(target), "v")
	if target == "" || target == current {
		return false
	}
	if policy.DesiredVersion != "" && strings.TrimPrefix(policy.DesiredVersion, "v") != target {
		return false
	}
	currentParts, currentOK := parseSemver(current)
	targetParts, targetOK := parseSemver(target)
	if !currentOK || !targetOK {
		return policy.Mode == "notify"
	}
	if targetParts[0] != currentParts[0] {
		return false
	}
	switch policy.Mode {
	case "minor":
		return targetParts[1] >= currentParts[1] && semverGreater(targetParts, currentParts)
	case "patch", "":
		return targetParts[1] == currentParts[1] && semverGreater(targetParts, currentParts)
	case "notify":
		return semverGreater(targetParts, currentParts)
	default:
		return false
	}
}

func parseSemver(raw string) ([3]int, bool) {
	var out [3]int
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "v"))
	parts := strings.Split(raw, ".")
	if len(parts) < 3 {
		return out, false
	}
	for index := 0; index < 3; index++ {
		value, err := strconv.Atoi(parts[index])
		if err != nil {
			return out, false
		}
		out[index] = value
	}
	return out, true
}

func semverGreater(next, current [3]int) bool {
	for index := 0; index < 3; index++ {
		if next[index] != current[index] {
			return next[index] > current[index]
		}
	}
	return false
}
