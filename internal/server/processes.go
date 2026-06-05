package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"gpufleet/internal/model"
)

type ProcessStore struct {
	path   string
	mu     sync.Mutex
	latest map[string]StoredProcessSnapshot
}

type StoredProcessSnapshot struct {
	DeviceID  string                `json:"device_id"`
	Timestamp time.Time             `json:"timestamp"`
	Process   model.ProcessSnapshot `json:"process"`
}

type processFile struct {
	UpdatedAt time.Time               `json:"updated_at"`
	Latest    []StoredProcessSnapshot `json:"latest"`
}

func NewProcessStore(path string) (*ProcessStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	store := &ProcessStore{
		path:   path,
		latest: map[string]StoredProcessSnapshot{},
	}
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return store, nil
}

func (s *ProcessStore) Replace(batch model.ProcessBatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, snapshot := range s.latest {
		if snapshot.DeviceID == batch.DeviceID {
			delete(s.latest, key)
		}
	}
	for _, process := range batch.Processes {
		key := processKey(batch.DeviceID, process.GPUID, process.PID)
		s.latest[key] = StoredProcessSnapshot{
			DeviceID:  batch.DeviceID,
			Timestamp: batch.Timestamp.UTC(),
			Process:   process,
		}
	}
	return s.saveLocked()
}

func (s *ProcessStore) Latest(deviceID, gpuID string) []StoredProcessSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]StoredProcessSnapshot, 0, len(s.latest))
	for _, snapshot := range s.latest {
		if deviceID != "" && snapshot.DeviceID != deviceID {
			continue
		}
		if gpuID != "" && snapshot.Process.GPUID != gpuID {
			continue
		}
		out = append(out, snapshot)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DeviceID == out[j].DeviceID {
			if out[i].Process.GPUID == out[j].Process.GPUID {
				return out[i].Process.PID < out[j].Process.PID
			}
			return out[i].Process.GPUID < out[j].Process.GPUID
		}
		return out[i].DeviceID < out[j].DeviceID
	})
	return out
}

func (s *ProcessStore) RemoveDevice(deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, snapshot := range s.latest {
		if snapshot.DeviceID == deviceID {
			delete(s.latest, key)
		}
	}
	return s.saveLocked()
}

func (s *ProcessStore) load() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var file processFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return err
	}
	for _, snapshot := range file.Latest {
		key := processKey(snapshot.DeviceID, snapshot.Process.GPUID, snapshot.Process.PID)
		s.latest[key] = snapshot
	}
	return nil
}

func (s *ProcessStore) saveLocked() error {
	file := processFile{
		UpdatedAt: time.Now().UTC(),
		Latest:    make([]StoredProcessSnapshot, 0, len(s.latest)),
	}
	for _, snapshot := range s.latest {
		file.Latest = append(file.Latest, snapshot)
	}
	sort.Slice(file.Latest, func(i, j int) bool {
		if file.Latest[i].DeviceID == file.Latest[j].DeviceID {
			if file.Latest[i].Process.GPUID == file.Latest[j].Process.GPUID {
				return file.Latest[i].Process.PID < file.Latest[j].Process.PID
			}
			return file.Latest[i].Process.GPUID < file.Latest[j].Process.GPUID
		}
		return file.Latest[i].DeviceID < file.Latest[j].DeviceID
	})
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func processKey(deviceID, gpuID string, pid int) string {
	return deviceID + "/" + gpuID + "/" + strconv.Itoa(pid)
}
