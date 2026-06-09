package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gpufleet/internal/model"

	"golang.org/x/crypto/pbkdf2"
)

type MetadataStore struct {
	path string
	mu   sync.Mutex
	data metadataFile
}

const metadataCurrentSchemaVersion = 1

type metadataFile struct {
	SchemaVersion       int                   `json:"schema_version"`
	CreatedAt           time.Time             `json:"created_at"`
	SetupComplete       *bool                 `json:"setup_complete,omitempty"`
	Admin               AdminAccount          `json:"admin"`
	Service             ServiceConfig         `json:"service"`
	Devices             map[string]*Device    `json:"devices"`
	WebSessions         map[string]WebSession `json:"web_sessions,omitempty"`
	GuestVisits         []GuestVisit          `json:"guest_visits,omitempty"`
	AuditEvents         []AuditEvent          `json:"audit_events"`
	LastProcessSet      map[string]time.Time  `json:"last_process_set,omitempty"`
	PendingUpdateNotice *UpdateNotice         `json:"pending_update_notice,omitempty"`
}

type AdminAccount struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	Salt         string `json:"salt"`
	Iterations   int    `json:"iterations"`
}

type ServiceConfig struct {
	Addr              string    `json:"addr"`
	Port              int       `json:"port"`
	HTTPS             bool      `json:"https"`
	Language          string    `json:"language,omitempty"`
	GuestEnabled      bool      `json:"guest_enabled,omitempty"`
	UpdateProxy       string    `json:"update_proxy,omitempty"`
	AutoUpdateEnabled *bool     `json:"auto_update_enabled,omitempty"`
	LegacyAgentAuth   bool      `json:"legacy_agent_auth_enabled,omitempty"`
	MinFreeBytes      uint64    `json:"min_free_bytes,omitempty"`
	EnergyPricePerKWh float64   `json:"energy_price_per_kwh,omitempty"`
	EnergyCurrency    string    `json:"energy_currency,omitempty"`
	ThermalHotCelsius float64   `json:"thermal_hot_celsius,omitempty"`
	IdleUtilPercent   float64   `json:"idle_utilization_percent,omitempty"`
	IdlePowerWatts    float64   `json:"idle_power_watts,omitempty"`
	CertPath          string    `json:"cert_path,omitempty"`
	KeyPath           string    `json:"key_path,omitempty"`
	CertNotAfter      time.Time `json:"cert_not_after,omitempty"`
	ConfigRevision    int       `json:"config_revision"`
	UpdatedAt         time.Time `json:"updated_at,omitempty"`
}

func (c ServiceConfig) AutoUpdateOn() bool {
	return c.AutoUpdateEnabled == nil || *c.AutoUpdateEnabled
}

func (c ServiceConfig) EnergySettings() EnergySettings {
	settings := EnergySettings{
		EnergyPricePerKWh:      c.EnergyPricePerKWh,
		EnergyCurrency:         strings.TrimSpace(c.EnergyCurrency),
		ThermalHotCelsius:      c.ThermalHotCelsius,
		IdleUtilizationPercent: c.IdleUtilPercent,
		IdlePowerWatts:         c.IdlePowerWatts,
	}
	if settings.EnergyPricePerKWh < 0 {
		settings.EnergyPricePerKWh = 0
	}
	if settings.EnergyCurrency == "" {
		settings.EnergyCurrency = defaultEnergyCurrency
	}
	if settings.ThermalHotCelsius <= 0 {
		settings.ThermalHotCelsius = defaultThermalHotCelsius
	}
	if settings.IdleUtilizationPercent <= 0 {
		settings.IdleUtilizationPercent = defaultIdleUtilizationPercent
	}
	if settings.IdlePowerWatts <= 0 {
		settings.IdlePowerWatts = defaultIdlePowerWatts
	}
	return settings
}

type UpdateNotice struct {
	ID              string    `json:"id"`
	Kind            string    `json:"kind"`
	Product         string    `json:"product,omitempty"`
	PreviousCommit  string    `json:"previous_commit,omitempty"`
	TargetCommit    string    `json:"target_commit,omitempty"`
	CurrentCommit   string    `json:"current_commit,omitempty"`
	PreviousVersion string    `json:"previous_version,omitempty"`
	CurrentVersion  string    `json:"current_version,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Summary         []string  `json:"summary,omitempty"`
	SummaryEN       []string  `json:"summary_en,omitempty"`
}

type Device struct {
	ID             string                   `json:"id"`
	Alias          string                   `json:"alias"`
	Secret         string                   `json:"secret,omitempty"`
	Enabled        bool                     `json:"enabled"`
	CreatedAt      time.Time                `json:"created_at"`
	LastSeenAt     time.Time                `json:"last_seen_at,omitempty"`
	AgentVersion   string                   `json:"agent_version,omitempty"`
	Hostname       string                   `json:"hostname,omitempty"`
	OS             string                   `json:"os,omitempty"`
	OSVersion      string                   `json:"os_version,omitempty"`
	GPUCount       int                      `json:"gpu_count"`
	LastSampleAt   time.Time                `json:"last_sample_at,omitempty"`
	LastError      string                   `json:"last_error,omitempty"`
	LastRemoteAddr string                   `json:"last_remote_addr,omitempty"`
	ConfigReport   *model.AgentConfigReport `json:"config_report,omitempty"`
}

type DeviceAuthInfo struct {
	Secret       string
	Enabled      bool
	AgentVersion string
}

type WebSession struct {
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type GuestVisit struct {
	At          time.Time `json:"at"`
	RemoteIP    string    `json:"remote_ip"`
	UserAgent   string    `json:"user_agent,omitempty"`
	Path        string    `json:"path,omitempty"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	Language    string    `json:"language,omitempty"`
	Platform    string    `json:"platform,omitempty"`
	Screen      string    `json:"screen,omitempty"`
	Timezone    string    `json:"timezone,omitempty"`
}

type AuditEvent struct {
	At        time.Time `json:"at"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Actor     string    `json:"actor,omitempty"`
	RemoteIP  string    `json:"remote_ip,omitempty"`
	DeviceID  string    `json:"device_id,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
}

func OpenMetadataStore(path string, adminPassword string, bootstrapDeviceID string, bootstrapSecret string) (*MetadataStore, string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, "", err
	}

	store := &MetadataStore{path: path}
	if err := store.load(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, "", err
		}
		store.data = metadataFile{
			SchemaVersion:  metadataCurrentSchemaVersion,
			CreatedAt:      time.Now().UTC(),
			Devices:        map[string]*Device{},
			WebSessions:    map[string]WebSession{},
			LastProcessSet: map[string]time.Time{},
		}
	}
	if store.data.Devices == nil {
		store.data.Devices = map[string]*Device{}
	}
	if store.data.WebSessions == nil {
		store.data.WebSessions = map[string]WebSession{}
	}
	if store.data.GuestVisits == nil {
		store.data.GuestVisits = []GuestVisit{}
	}
	if store.data.LastProcessSet == nil {
		store.data.LastProcessSet = map[string]time.Time{}
	}

	generatedAdminPassword := ""
	if store.data.Admin.PasswordHash == "" && adminPassword != "" {
		account, err := NewAdminAccount("admin", adminPassword)
		if err != nil {
			return nil, "", err
		}
		store.data.Admin = account
		store.addAuditLocked("admin_created", "created initial admin account")
	}
	if store.data.Admin.Username == "" && store.data.Admin.PasswordHash != "" {
		store.data.Admin.Username = "admin"
	}
	if store.data.SetupComplete == nil {
		complete := store.data.Admin.PasswordHash != ""
		store.data.SetupComplete = &complete
	}

	if bootstrapDeviceID != "" && bootstrapSecret != "" {
		if _, exists := store.data.Devices[bootstrapDeviceID]; !exists {
			store.data.Devices[bootstrapDeviceID] = &Device{
				ID:        bootstrapDeviceID,
				Alias:     bootstrapDeviceID,
				Secret:    bootstrapSecret,
				Enabled:   true,
				CreatedAt: time.Now().UTC(),
			}
			store.addAuditLocked("device_created", fmt.Sprintf("created bootstrap device %s", bootstrapDeviceID))
		}
	}

	if err := store.saveLocked(); err != nil {
		return nil, "", err
	}
	return store, generatedAdminPassword, nil
}

func (s *MetadataStore) EnsureServiceConfig(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := s.ensureServiceConfigLocked(addr)
	if !changed {
		return nil
	}
	return s.saveLocked()
}

func (s *MetadataStore) ensureServiceConfigLocked(addr string) bool {
	changed := false
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	if s.data.Service.Addr == "" {
		s.data.Service.Addr = addr
		changed = true
	}
	if s.data.Service.Port == 0 {
		if port, ok := portFromAddr(s.data.Service.Addr); ok {
			s.data.Service.Port = port
			changed = true
		}
	}
	if s.data.Service.Language == "" {
		s.data.Service.Language = defaultLanguage
		changed = true
	}
	if s.data.Service.UpdatedAt.IsZero() {
		s.data.Service.UpdatedAt = time.Now().UTC()
		changed = true
	}
	return changed
}

func (s *MetadataStore) SetupComplete() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setupCompleteLocked()
}

func (s *MetadataStore) setupCompleteLocked() bool {
	return s.data.SetupComplete != nil && *s.data.SetupComplete
}

func (s *MetadataStore) HasAdminPassword() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Admin.PasswordHash != ""
}

func (s *MetadataStore) ServiceConfig() ServiceConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Service
}

func (s *MetadataStore) CertificateFiles() (string, string, bool) {
	s.mu.Lock()
	config := s.data.Service
	s.mu.Unlock()
	if !config.HTTPS || config.CertPath == "" || config.KeyPath == "" {
		return "", "", false
	}
	return s.resolveDataPath(config.CertPath), s.resolveDataPath(config.KeyPath), true
}

func (s *MetadataStore) CompleteInitialSetup(password string, port int, language string, certPEM, keyPEM []byte) (ServiceConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Admin.PasswordHash != "" || s.setupCompleteLocked() {
		return ServiceConfig{}, errors.New("setup already completed")
	}
	if err := validatePassword(password); err != nil {
		return ServiceConfig{}, err
	}
	if port != 0 {
		if err := validatePort(port); err != nil {
			return ServiceConfig{}, err
		}
	}
	account, err := NewAdminAccount("admin", password)
	if err != nil {
		return ServiceConfig{}, err
	}
	s.data.Admin = account
	s.updatePortLocked(port)
	if err := s.updateLanguageLocked(language); err != nil {
		return ServiceConfig{}, err
	}
	if len(certPEM) > 0 || len(keyPEM) > 0 {
		if err := s.saveCertificateLocked(certPEM, keyPEM); err != nil {
			return ServiceConfig{}, err
		}
	}
	complete := true
	s.data.SetupComplete = &complete
	s.bumpServiceConfigLocked()
	s.addAuditLocked("setup_completed", "completed initial setup")
	return s.data.Service, s.saveLocked()
}

func (s *MetadataStore) ReconfigureSetup(password string, port int, language string, certPEM, keyPEM []byte) (ServiceConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if password != "" {
		if err := validatePassword(password); err != nil {
			return ServiceConfig{}, err
		}
		account, err := NewAdminAccount("admin", password)
		if err != nil {
			return ServiceConfig{}, err
		}
		s.data.Admin = account
		s.addAuditLocked("admin_password_replaced", "replaced admin password from setup wizard")
	}
	if port != 0 {
		if err := validatePort(port); err != nil {
			return ServiceConfig{}, err
		}
		s.updatePortLocked(port)
	}
	if err := s.updateLanguageLocked(language); err != nil {
		return ServiceConfig{}, err
	}
	if len(certPEM) > 0 || len(keyPEM) > 0 {
		if err := s.saveCertificateLocked(certPEM, keyPEM); err != nil {
			return ServiceConfig{}, err
		}
		s.addAuditLocked("service_certificate_uploaded", "uploaded HTTPS certificate from setup wizard")
	}
	complete := true
	s.data.SetupComplete = &complete
	s.bumpServiceConfigLocked()
	s.addAuditLocked("setup_reconfigured", "applied setup wizard configuration")
	return s.data.Service, s.saveLocked()
}

func (s *MetadataStore) UpdatePassword(currentPassword, nextPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Admin.PasswordHash == "" {
		return errors.New("admin password is not configured")
	}
	if ok, _ := verifyPassword(currentPassword, s.data.Admin); !ok {
		return errors.New("current password is incorrect")
	}
	if err := validatePassword(nextPassword); err != nil {
		return err
	}
	account, err := NewAdminAccount("admin", nextPassword)
	if err != nil {
		return err
	}
	s.data.Admin = account
	s.addAuditLocked("admin_password_changed", "changed admin password")
	return s.saveLocked()
}

func (s *MetadataStore) ReplacePassword(nextPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validatePassword(nextPassword); err != nil {
		return err
	}
	account, err := NewAdminAccount("admin", nextPassword)
	if err != nil {
		return err
	}
	s.data.Admin = account
	complete := true
	s.data.SetupComplete = &complete
	s.addAuditLocked("admin_password_replaced", "replaced admin password from setup wizard")
	return s.saveLocked()
}

func (s *MetadataStore) UpdateServicePort(port int) (ServiceConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validatePort(port); err != nil {
		return ServiceConfig{}, err
	}
	s.updatePortLocked(port)
	s.bumpServiceConfigLocked()
	s.addAuditLocked("service_port_changed", fmt.Sprintf("configured service port %d", port))
	return s.data.Service, s.saveLocked()
}

func (s *MetadataStore) UpdateMinFreeBytes(minFreeBytes uint64) (ServiceConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if minFreeBytes == 0 {
		return ServiceConfig{}, errors.New("minimum disk reserve must be greater than 0")
	}
	s.data.Service.MinFreeBytes = minFreeBytes
	s.bumpServiceConfigLocked()
	s.addAuditLocked("service_disk_reserve_changed", fmt.Sprintf("changed minimum free disk reserve to %d bytes", minFreeBytes))
	return s.data.Service, s.saveLocked()
}

func (s *MetadataStore) UpdateLanguage(language string) (ServiceConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.updateLanguageLocked(language); err != nil {
		return ServiceConfig{}, err
	}
	s.bumpServiceConfigLocked()
	s.addAuditLocked("service_language_changed", fmt.Sprintf("changed UI language to %s", s.data.Service.Language))
	return s.data.Service, s.saveLocked()
}

func (s *MetadataStore) UpdateGuestEnabled(enabled bool) (ServiceConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Service.GuestEnabled = enabled
	s.bumpServiceConfigLocked()
	if enabled {
		s.addAuditLocked("guest_access_enabled", "enabled guest overview access")
	} else {
		s.addAuditLocked("guest_access_disabled", "disabled guest overview access")
	}
	return s.data.Service, s.saveLocked()
}

func (s *MetadataStore) UpdateProxy(proxyURL string) (ServiceConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	proxyURL, err := normalizeProxyURL(proxyURL)
	if err != nil {
		return ServiceConfig{}, err
	}
	s.data.Service.UpdateProxy = proxyURL
	s.bumpServiceConfigLocked()
	if proxyURL == "" {
		s.addAuditLocked("update_proxy_changed", "cleared update proxy")
	} else {
		s.addAuditLocked("update_proxy_changed", "changed update proxy")
	}
	return s.data.Service, s.saveLocked()
}

func (s *MetadataStore) UpdateAutoUpdateEnabled(enabled bool) (ServiceConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Service.AutoUpdateEnabled = &enabled
	s.bumpServiceConfigLocked()
	if enabled {
		s.addAuditLocked("auto_update_enabled", "enabled automatic update checks")
	} else {
		s.addAuditLocked("auto_update_disabled", "disabled automatic update checks")
	}
	return s.data.Service, s.saveLocked()
}

func (s *MetadataStore) UpdateLegacyAgentAuthEnabled(enabled bool) (ServiceConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Service.LegacyAgentAuth = enabled
	s.bumpServiceConfigLocked()
	if enabled {
		s.addAuditLocked("legacy_agent_auth_enabled", "enabled legacy Agent HMAC compatibility")
	} else {
		s.addAuditLocked("legacy_agent_auth_disabled", "disabled legacy Agent HMAC compatibility")
	}
	return s.data.Service, s.saveLocked()
}

func (s *MetadataStore) UpdateEnergySettings(settings EnergySettings) (ServiceConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateEnergySettings(settings); err != nil {
		return ServiceConfig{}, err
	}
	s.data.Service.EnergyPricePerKWh = settings.EnergyPricePerKWh
	s.data.Service.EnergyCurrency = normalizeEnergyCurrency(settings.EnergyCurrency)
	s.data.Service.ThermalHotCelsius = settings.ThermalHotCelsius
	s.data.Service.IdleUtilPercent = settings.IdleUtilizationPercent
	s.data.Service.IdlePowerWatts = settings.IdlePowerWatts
	s.bumpServiceConfigLocked()
	s.addAuditLocked("energy_display_settings_changed", "changed energy and thermal display settings")
	return s.data.Service, s.saveLocked()
}

func (s *MetadataStore) SaveUpdateNotice(notice UpdateNotice) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if notice.ID == "" {
		notice.ID = strconv.FormatInt(time.Now().UTC().UnixNano(), 36)
	}
	if notice.Kind == "" {
		notice.Kind = "auto_update"
	}
	if notice.UpdatedAt.IsZero() {
		notice.UpdatedAt = time.Now().UTC()
	}
	s.data.PendingUpdateNotice = &notice
	return s.saveLocked()
}

func (s *MetadataStore) TakeUpdateNotice() *UpdateNotice {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.PendingUpdateNotice == nil {
		return nil
	}
	notice := *s.data.PendingUpdateNotice
	s.data.PendingUpdateNotice = nil
	_ = s.saveLocked()
	return &notice
}

func (s *MetadataStore) PeekUpdateNotice() *UpdateNotice {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.PendingUpdateNotice == nil {
		return nil
	}
	notice := *s.data.PendingUpdateNotice
	return &notice
}

func (s *MetadataStore) RecordGuestVisit(visit GuestVisit) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	visit.At = visit.At.UTC()
	if visit.At.IsZero() {
		visit.At = time.Now().UTC()
	}
	visit.UserAgent = limitText(strings.TrimSpace(visit.UserAgent), 240)
	visit.Path = limitText(strings.TrimSpace(visit.Path), 120)
	visit.Fingerprint = limitText(strings.TrimSpace(visit.Fingerprint), 80)
	visit.Language = limitText(strings.TrimSpace(visit.Language), 80)
	visit.Platform = limitText(strings.TrimSpace(visit.Platform), 80)
	visit.Screen = limitText(strings.TrimSpace(visit.Screen), 80)
	visit.Timezone = limitText(strings.TrimSpace(visit.Timezone), 80)
	s.data.GuestVisits = append(s.data.GuestVisits, GuestVisit{
		At:          visit.At,
		RemoteIP:    visit.RemoteIP,
		UserAgent:   visit.UserAgent,
		Path:        visit.Path,
		Fingerprint: visit.Fingerprint,
		Language:    visit.Language,
		Platform:    visit.Platform,
		Screen:      visit.Screen,
		Timezone:    visit.Timezone,
	})
	if len(s.data.GuestVisits) > 300 {
		s.data.GuestVisits = s.data.GuestVisits[len(s.data.GuestVisits)-300:]
	}
	return s.saveLocked()
}

func (s *MetadataStore) GuestVisits(limit int) []GuestVisit {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > 300 {
		limit = 100
	}
	start := len(s.data.GuestVisits) - limit
	if start < 0 {
		start = 0
	}
	visits := append([]GuestVisit(nil), s.data.GuestVisits[start:]...)
	sort.Slice(visits, func(i, j int) bool { return visits[i].At.After(visits[j].At) })
	return visits
}

func (s *MetadataStore) RecentAuditEvents(limit int) []AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	start := len(s.data.AuditEvents) - limit
	if start < 0 {
		start = 0
	}
	events := append([]AuditEvent(nil), s.data.AuditEvents[start:]...)
	sort.Slice(events, func(i, j int) bool { return events[i].At.After(events[j].At) })
	return events
}

func (s *MetadataStore) SaveCertificate(certPEM, keyPEM []byte) (ServiceConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.saveCertificateLocked(certPEM, keyPEM); err != nil {
		return ServiceConfig{}, err
	}
	s.bumpServiceConfigLocked()
	s.addAuditLocked("service_certificate_uploaded", "uploaded HTTPS certificate")
	return s.data.Service, s.saveLocked()
}

func (s *MetadataStore) saveCertificateLocked(certPEM, keyPEM []byte) error {
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return errors.New("certificate and private key are required")
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("invalid certificate pair: %w", err)
	}
	if len(pair.Certificate) == 0 {
		return errors.New("certificate is empty")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return fmt.Errorf("invalid certificate: %w", err)
	}
	certDir := s.resolveDataPath("certs")
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return err
	}
	certPath := filepath.Join(certDir, "server.crt")
	keyPath := filepath.Join(certDir, "server.key")
	if err := writeFileAtomicSync(certPath, certPEM, 0600); err != nil {
		return err
	}
	if err := writeFileAtomicSync(keyPath, keyPEM, 0600); err != nil {
		return err
	}
	s.data.Service.HTTPS = true
	s.data.Service.CertPath = filepath.ToSlash(filepath.Join("certs", "server.crt"))
	s.data.Service.KeyPath = filepath.ToSlash(filepath.Join("certs", "server.key"))
	s.data.Service.CertNotAfter = leaf.NotAfter.UTC()
	return nil
}

func (s *MetadataStore) updatePortLocked(port int) {
	if port <= 0 {
		return
	}
	if s.data.Service.Addr == "" {
		s.data.Service.Addr = fmt.Sprintf("127.0.0.1:%d", port)
	}
	host, _, err := splitHostPort(s.data.Service.Addr)
	if err != nil {
		host = "127.0.0.1"
	}
	s.data.Service.Port = port
	s.data.Service.Addr = fmt.Sprintf("%s:%d", host, port)
}

func (s *MetadataStore) updateLanguageLocked(language string) error {
	language = normalizeLanguage(language)
	if language == "" {
		return nil
	}
	if !validLanguage(language) {
		return fmt.Errorf("unsupported language %q", language)
	}
	s.data.Service.Language = language
	return nil
}

func (s *MetadataStore) bumpServiceConfigLocked() {
	s.data.Service.ConfigRevision++
	s.data.Service.UpdatedAt = time.Now().UTC()
}

func (s *MetadataStore) resolveDataPath(rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(filepath.Dir(s.path), filepath.FromSlash(rel))
}

func (s *MetadataStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return err
	}
	s.migrateLocked()
	return nil
}

func (s *MetadataStore) migrateLocked() {
	if s.data.SchemaVersion <= 0 {
		s.data.SchemaVersion = 1
	}
	if s.data.CreatedAt.IsZero() {
		s.data.CreatedAt = time.Now().UTC()
	}
	if s.data.Devices == nil {
		s.data.Devices = map[string]*Device{}
	}
	if s.data.WebSessions == nil {
		s.data.WebSessions = map[string]WebSession{}
	}
	if s.data.GuestVisits == nil {
		s.data.GuestVisits = []GuestVisit{}
	}
	if s.data.LastProcessSet == nil {
		s.data.LastProcessSet = map[string]time.Time{}
	}
	if s.data.Admin.Username == "" && s.data.Admin.PasswordHash != "" {
		s.data.Admin.Username = "admin"
	}
	if s.data.SetupComplete == nil {
		complete := s.data.Admin.PasswordHash != ""
		s.data.SetupComplete = &complete
	}
	s.data.SchemaVersion = metadataCurrentSchemaVersion
}

func (s *MetadataStore) saveLocked() error {
	s.data.SchemaVersion = metadataCurrentSchemaVersion
	return writeJSONFileAtomicSync(s.path, s.data, 0600)
}

func (s *MetadataStore) SaveWebSession(tokenHash string, createdAt, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneWebSessionsLocked(time.Now().UTC())
	s.data.WebSessions[tokenHash] = WebSession{
		CreatedAt:  createdAt.UTC(),
		LastSeenAt: createdAt.UTC(),
		ExpiresAt:  expiresAt.UTC(),
	}
	return s.saveLocked()
}

func (s *MetadataStore) WebSession(tokenHash string, now time.Time) (WebSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.data.WebSessions[tokenHash]
	if !ok {
		return WebSession{}, false
	}
	if !now.UTC().Before(session.ExpiresAt) {
		delete(s.data.WebSessions, tokenHash)
		_ = s.saveLocked()
		return WebSession{}, false
	}
	session.LastSeenAt = now.UTC()
	s.data.WebSessions[tokenHash] = session
	_ = s.saveLocked()
	return session, true
}

func (s *MetadataStore) DeleteWebSession(tokenHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data.WebSessions[tokenHash]; !ok {
		return nil
	}
	delete(s.data.WebSessions, tokenHash)
	return s.saveLocked()
}

func (s *MetadataStore) pruneWebSessionsLocked(now time.Time) {
	for tokenHash, session := range s.data.WebSessions {
		if !now.Before(session.ExpiresAt) {
			delete(s.data.WebSessions, tokenHash)
		}
	}
}

func (s *MetadataStore) DeviceSecret(deviceID string) (string, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.data.Devices[deviceID]
	if !ok {
		return "", false, false
	}
	return device.Secret, device.Enabled, true
}

func (s *MetadataStore) DeviceAuth(deviceID string) (DeviceAuthInfo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.data.Devices[deviceID]
	if !ok {
		return DeviceAuthInfo{}, false
	}
	return DeviceAuthInfo{
		Secret:       device.Secret,
		Enabled:      device.Enabled,
		AgentVersion: device.AgentVersion,
	}, true
}

func (s *MetadataStore) UpdateHeartbeat(deviceID string, update func(*Device)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.data.Devices[deviceID]
	if !ok {
		return errors.New("device not found")
	}
	update(device)
	device.LastSeenAt = time.Now().UTC()
	return s.saveLocked()
}

func (s *MetadataStore) RecordSample(deviceID string, sampleAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.data.Devices[deviceID]
	if !ok {
		return errors.New("device not found")
	}
	device.LastSeenAt = time.Now().UTC()
	device.LastSampleAt = sampleAt
	return s.saveLocked()
}

func (s *MetadataStore) RecordAgentConfig(deviceID string, report model.AgentConfigReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.data.Devices[deviceID]
	if !ok {
		return errors.New("device not found")
	}
	now := time.Now().UTC()
	if report.CollectedAt.IsZero() {
		report.CollectedAt = now
	}
	report.DeviceID = deviceID
	device.ConfigReport = &report
	device.LastSeenAt = now
	s.addAuditLocked("device_config_reported", fmt.Sprintf("stored Agent configuration report for %s", deviceID))
	return s.saveLocked()
}

func (s *MetadataStore) ListDevices() []Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Device, 0, len(s.data.Devices))
	for _, device := range s.data.Devices {
		out = append(out, *device)
	}
	return out
}

func (s *MetadataStore) CreateDevice(alias string) (Device, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := "device_" + time.Now().UTC().Format("20060102150405")
	for {
		if _, exists := s.data.Devices[id]; !exists {
			break
		}
		suffix, err := randomHex(3)
		if err != nil {
			return Device{}, "", err
		}
		id = "device_" + time.Now().UTC().Format("20060102150405") + "_" + suffix
	}
	if alias == "" {
		alias = id
	}
	secret, err := randomHex(24)
	if err != nil {
		return Device{}, "", err
	}
	device := Device{
		ID:        id,
		Alias:     alias,
		Secret:    secret,
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
	}
	s.data.Devices[id] = &device
	s.addAuditLocked("device_created", fmt.Sprintf("created device %s", id))
	return device, secret, s.saveLocked()
}

func (s *MetadataStore) SetDeviceEnabled(deviceID string, enabled bool) (Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.data.Devices[deviceID]
	if !ok {
		return Device{}, errors.New("device not found")
	}
	device.Enabled = enabled
	action := "device_disabled"
	if enabled {
		action = "device_enabled"
	}
	s.addAuditLocked(action, fmt.Sprintf("%s device %s", action, deviceID))
	return *device, s.saveLocked()
}

func (s *MetadataStore) RenameDevice(deviceID string, alias string) (Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.data.Devices[deviceID]
	if !ok {
		return Device{}, errors.New("device not found")
	}
	alias = strings.TrimSpace(alias)
	if alias == "" {
		alias = deviceID
	}
	if len(alias) > 96 {
		return Device{}, errors.New("device alias is too long")
	}
	device.Alias = alias
	s.addAuditLocked("device_renamed", fmt.Sprintf("renamed device %s", deviceID))
	return *device, s.saveLocked()
}

func (s *MetadataStore) DeleteDevice(deviceID string) (Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.data.Devices[deviceID]
	if !ok {
		return Device{}, errors.New("device not found")
	}
	deleted := *device
	delete(s.data.Devices, deviceID)
	delete(s.data.LastProcessSet, deviceID)
	s.addAuditLocked("device_deleted", fmt.Sprintf("deleted device %s", deviceID))
	return deleted, s.saveLocked()
}

func (s *MetadataStore) RotateDeviceSecret(deviceID string) (Device, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device, ok := s.data.Devices[deviceID]
	if !ok {
		return Device{}, "", errors.New("device not found")
	}
	secret, err := randomHex(24)
	if err != nil {
		return Device{}, "", err
	}
	device.Secret = secret
	s.addAuditLocked("device_secret_rotated", fmt.Sprintf("rotated device secret for %s", deviceID))
	return *device, secret, s.saveLocked()
}

func (s *MetadataStore) VerifyAdmin(password string) bool {
	s.mu.Lock()
	account := s.data.Admin
	s.mu.Unlock()
	if account.PasswordHash == "" {
		return false
	}
	ok, needsUpgrade := verifyPassword(password, account)
	if !ok {
		return false
	}
	if needsUpgrade {
		upgraded, err := NewAdminAccount(account.Username, password)
		if err == nil {
			s.mu.Lock()
			if s.data.Admin.PasswordHash == account.PasswordHash {
				s.data.Admin = upgraded
				s.addAuditLocked("admin_password_hash_upgraded", "upgraded admin password hash to PBKDF2-SHA256")
				_ = s.saveLocked()
			}
			s.mu.Unlock()
		}
	}
	return true
}

func (s *MetadataStore) AddAudit(eventType, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addAuditLocked(eventType, message)
	if len(s.data.AuditEvents) > 1000 {
		s.data.AuditEvents = s.data.AuditEvents[len(s.data.AuditEvents)-1000:]
	}
	return s.saveLocked()
}

func (s *MetadataStore) AddAuditEvent(event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	s.data.AuditEvents = append(s.data.AuditEvents, event)
	if len(s.data.AuditEvents) > 1000 {
		s.data.AuditEvents = s.data.AuditEvents[len(s.data.AuditEvents)-1000:]
	}
	return s.saveLocked()
}

func (s *MetadataStore) addAuditLocked(eventType, message string) {
	s.data.AuditEvents = append(s.data.AuditEvents, AuditEvent{
		At:      time.Now().UTC(),
		Type:    eventType,
		Message: message,
	})
}

func NewAdminAccount(username, password string) (AdminAccount, error) {
	salt, err := randomHex(16)
	if err != nil {
		return AdminAccount{}, err
	}
	account := AdminAccount{
		Username:   username,
		Salt:       salt,
		Iterations: passwordPBKDF2Iterations,
	}
	account.PasswordHash = derivePBKDF2Password(password, account.Salt, account.Iterations)
	return account, nil
}

const (
	passwordHashPBKDF2Prefix = "pbkdf2-sha256"
	passwordPBKDF2Iterations = 210000
)

func verifyPassword(password string, account AdminAccount) (bool, bool) {
	if strings.HasPrefix(account.PasswordHash, passwordHashPBKDF2Prefix+"$") {
		derived, iterations, ok := derivePBKDF2PasswordForStoredHash(password, account.PasswordHash)
		if !ok {
			return false, false
		}
		matched := subtle.ConstantTimeCompare([]byte(derived), []byte(account.PasswordHash)) == 1
		return matched, matched && iterations < passwordPBKDF2Iterations
	}
	derived := deriveLegacyPassword(password, account.Salt, account.Iterations)
	matched := subtle.ConstantTimeCompare([]byte(derived), []byte(account.PasswordHash)) == 1
	return matched, matched
}

func derivePBKDF2Password(password, salt string, iterations int) string {
	if iterations <= 0 {
		iterations = passwordPBKDF2Iterations
	}
	saltBytes, err := hex.DecodeString(salt)
	if err != nil {
		saltBytes = []byte(salt)
	}
	key := pbkdf2.Key([]byte(password), saltBytes, iterations, 32, sha256.New)
	return fmt.Sprintf("%s$%d$%s$%s", passwordHashPBKDF2Prefix, iterations, salt, hex.EncodeToString(key))
}

func derivePBKDF2PasswordForStoredHash(password, stored string) (string, int, bool) {
	parts := strings.Split(stored, "$")
	if len(parts) != 4 || parts[0] != passwordHashPBKDF2Prefix {
		return "", 0, false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return "", 0, false
	}
	return derivePBKDF2Password(password, parts[2], iterations), iterations, true
}

func deriveLegacyPassword(password, salt string, iterations int) string {
	if iterations <= 0 {
		iterations = 120000
	}
	macInput := []byte(password + ":" + salt)
	sum := sha256.Sum256(macInput)
	current := sum[:]
	for i := 1; i < iterations; i++ {
		next := sha256.Sum256(append(current, macInput...))
		current = next[:]
	}
	return hex.EncodeToString(current)
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

const defaultLanguage = "zh-CN"

func normalizeLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "", "zh", "zh-cn", "zh_cn", "cn":
		if strings.TrimSpace(language) == "" {
			return ""
		}
		return "zh-CN"
	case "en", "en-us", "en_us":
		return "en-US"
	default:
		return strings.TrimSpace(language)
	}
}

func validLanguage(language string) bool {
	switch language {
	case "zh-CN", "en-US":
		return true
	default:
		return false
	}
}

func normalizeProxyURL(proxyURL string) (string, error) {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return "", nil
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("proxy URL must include scheme and host")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5", "socks5h":
		return proxyURL, nil
	default:
		return "", errors.New("proxy URL scheme must be http, https, socks5, or socks5h")
	}
}

func portFromAddr(addr string) (int, bool) {
	_, rawPort, err := splitHostPort(addr)
	if err != nil {
		return 0, false
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 {
		return 0, false
	}
	return port, true
}

func splitHostPort(addr string) (string, string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err == nil {
		return host, port, nil
	}
	if strings.Count(addr, ":") == 1 && !strings.HasPrefix(addr, "[") {
		parts := strings.SplitN(addr, ":", 2)
		return parts[0], parts[1], nil
	}
	return "", "", err
}
