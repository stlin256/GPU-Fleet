package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type MetadataStore struct {
	path string
	mu   sync.Mutex
	data metadataFile
}

type metadataFile struct {
	CreatedAt      time.Time            `json:"created_at"`
	Admin          AdminAccount         `json:"admin"`
	Devices        map[string]*Device   `json:"devices"`
	AuditEvents    []AuditEvent         `json:"audit_events"`
	LastProcessSet map[string]time.Time `json:"last_process_set,omitempty"`
}

type AdminAccount struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	Salt         string `json:"salt"`
	Iterations   int    `json:"iterations"`
}

type Device struct {
	ID             string    `json:"id"`
	Alias          string    `json:"alias"`
	Secret         string    `json:"secret,omitempty"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	LastSeenAt     time.Time `json:"last_seen_at,omitempty"`
	AgentVersion   string    `json:"agent_version,omitempty"`
	Hostname       string    `json:"hostname,omitempty"`
	OS             string    `json:"os,omitempty"`
	OSVersion      string    `json:"os_version,omitempty"`
	GPUCount       int       `json:"gpu_count"`
	LastSampleAt   time.Time `json:"last_sample_at,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	LastRemoteAddr string    `json:"last_remote_addr,omitempty"`
}

type AuditEvent struct {
	At      time.Time `json:"at"`
	Type    string    `json:"type"`
	Message string    `json:"message"`
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
			CreatedAt:      time.Now().UTC(),
			Devices:        map[string]*Device{},
			LastProcessSet: map[string]time.Time{},
		}
	}
	if store.data.Devices == nil {
		store.data.Devices = map[string]*Device{}
	}
	if store.data.LastProcessSet == nil {
		store.data.LastProcessSet = map[string]time.Time{}
	}

	generatedAdminPassword := ""
	if store.data.Admin.Username == "" {
		if adminPassword == "" {
			token, err := randomHex(18)
			if err != nil {
				return nil, "", err
			}
			adminPassword = token
			generatedAdminPassword = token
		}
		account, err := NewAdminAccount("admin", adminPassword)
		if err != nil {
			return nil, "", err
		}
		store.data.Admin = account
		store.addAuditLocked("admin_created", "created initial admin account")
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

func (s *MetadataStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, &s.data)
}

func (s *MetadataStore) saveLocked() error {
	tmp := s.path + ".tmp"
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
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

func (s *MetadataStore) VerifyAdmin(username, password string) bool {
	s.mu.Lock()
	account := s.data.Admin
	s.mu.Unlock()
	if username != account.Username || account.PasswordHash == "" {
		return false
	}
	return verifyPassword(password, account)
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
		Iterations: 120000,
	}
	account.PasswordHash = derivePassword(password, account.Salt, account.Iterations)
	return account, nil
}

func verifyPassword(password string, account AdminAccount) bool {
	return derivePassword(password, account.Salt, account.Iterations) == account.PasswordHash
}

func derivePassword(password, salt string, iterations int) string {
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
