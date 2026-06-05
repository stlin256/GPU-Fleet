package server

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gpufleet/internal/disk"
	"gpufleet/internal/model"
)

var ErrInsufficientStorage = errors.New("insufficient storage")

type MetricsStore struct {
	dir         string
	minFree     uint64
	retention   time.Duration
	mu          sync.Mutex
	latest      map[string]StoredGPU
	lastCleanup time.Time
}

type StoredSample struct {
	DeviceID     string            `json:"device_id"`
	AgentVersion string            `json:"agent_version"`
	Timestamp    time.Time         `json:"timestamp"`
	GPUs         []model.GPUStatus `json:"gpus"`
}

type StoredGPU struct {
	DeviceID  string          `json:"device_id"`
	Timestamp time.Time       `json:"timestamp"`
	GPU       model.GPUStatus `json:"gpu"`
}

type SeriesPoint struct {
	Timestamp             time.Time `json:"timestamp"`
	UtilizationGPUPercent *float64  `json:"utilization_gpu_percent,omitempty"`
	MemoryUsedBytes       uint64    `json:"memory_used_bytes"`
	MemoryTotalBytes      uint64    `json:"memory_total_bytes"`
	TemperatureCelsius    *float64  `json:"temperature_celsius,omitempty"`
	PowerDrawWatts        *float64  `json:"power_draw_watts,omitempty"`
}

type GPUStats struct {
	DeviceID                  string    `json:"device_id"`
	GPUID                     string    `json:"gpu_id"`
	GPUName                   string    `json:"gpu_name"`
	SampleCount               int       `json:"sample_count"`
	FirstSampleAt             time.Time `json:"first_sample_at"`
	LastSampleAt              time.Time `json:"last_sample_at"`
	AverageUtilizationPercent *float64  `json:"average_utilization_percent,omitempty"`
	PeakUtilizationPercent    *float64  `json:"peak_utilization_percent,omitempty"`
	IdleSampleCount           int       `json:"idle_sample_count"`
	IdleSamplePercent         float64   `json:"idle_sample_percent"`
	AverageMemoryUsedBytes    uint64    `json:"average_memory_used_bytes"`
	PeakMemoryUsedBytes       uint64    `json:"peak_memory_used_bytes"`
	MemoryTotalBytes          uint64    `json:"memory_total_bytes"`
	PeakTemperatureCelsius    *float64  `json:"peak_temperature_celsius,omitempty"`
	PeakPowerDrawWatts        *float64  `json:"peak_power_draw_watts,omitempty"`
}

func NewMetricsStore(dir string, minFreeBytes uint64, retention time.Duration) (*MetricsStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	store := &MetricsStore{
		dir:       dir,
		minFree:   minFreeBytes,
		retention: retention,
		latest:    map[string]StoredGPU{},
	}
	if err := store.loadLatest(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *MetricsStore) AppendBatch(batch model.SampleBatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if time.Since(s.lastCleanup) > time.Hour {
		if err := s.cleanupLocked(); err != nil {
			return err
		}
		s.lastCleanup = time.Now()
	}
	if err := s.ensureWritable(); err != nil {
		return err
	}

	bySegment := map[string][]StoredSample{}
	for _, sample := range batch.Samples {
		stored := StoredSample{
			DeviceID:     batch.DeviceID,
			AgentVersion: batch.AgentVersion,
			Timestamp:    sample.Timestamp.UTC(),
			GPUs:         sample.GPUs,
		}
		segment := stored.Timestamp.Format("2006010215")
		bySegment[segment] = append(bySegment[segment], stored)
		for _, gpu := range stored.GPUs {
			key := latestKey(batch.DeviceID, gpu.GPUID)
			current, ok := s.latest[key]
			if !ok || stored.Timestamp.After(current.Timestamp) {
				s.latest[key] = StoredGPU{DeviceID: batch.DeviceID, Timestamp: stored.Timestamp, GPU: gpu}
			}
		}
	}

	for segment, samples := range bySegment {
		if err := s.appendSegmentLocked(segment, samples); err != nil {
			return err
		}
	}
	return nil
}

func (s *MetricsStore) Latest() []StoredGPU {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]StoredGPU, 0, len(s.latest))
	for _, gpu := range s.latest {
		out = append(out, gpu)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DeviceID == out[j].DeviceID {
			return out[i].GPU.GPUID < out[j].GPU.GPUID
		}
		return out[i].DeviceID < out[j].DeviceID
	})
	return out
}

func (s *MetricsStore) RemoveDevice(deviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, gpu := range s.latest {
		if gpu.DeviceID == deviceID {
			delete(s.latest, key)
		}
	}
}

func (s *MetricsStore) Series(deviceID, gpuID string, since time.Time) ([]SeriesPoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	files, err := s.segmentFiles()
	if err != nil {
		return nil, err
	}
	points := []SeriesPoint{}
	for _, path := range files {
		if !segmentMayOverlap(path, since) {
			continue
		}
		if err := scanSegment(path, func(sample StoredSample) {
			if sample.DeviceID != deviceID || sample.Timestamp.Before(since) {
				return
			}
			for _, gpu := range sample.GPUs {
				if gpu.GPUID != gpuID {
					continue
				}
				points = append(points, SeriesPoint{
					Timestamp:             sample.Timestamp,
					UtilizationGPUPercent: gpu.UtilizationGPUPercent,
					MemoryUsedBytes:       gpu.MemoryUsedBytes,
					MemoryTotalBytes:      gpu.MemoryTotalBytes,
					TemperatureCelsius:    gpu.TemperatureCelsius,
					PowerDrawWatts:        gpu.PowerDrawWatts,
				})
			}
		}); err != nil {
			return nil, err
		}
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})
	return points, nil
}

func (s *MetricsStore) Stats(deviceID string, since time.Time) ([]GPUStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	files, err := s.segmentFiles()
	if err != nil {
		return nil, err
	}
	accumulators := map[string]*gpuStatsAccumulator{}
	for _, path := range files {
		if !segmentMayOverlap(path, since) {
			continue
		}
		if err := scanSegment(path, func(sample StoredSample) {
			if sample.Timestamp.Before(since) {
				return
			}
			if deviceID != "" && sample.DeviceID != deviceID {
				return
			}
			for _, gpu := range sample.GPUs {
				key := latestKey(sample.DeviceID, gpu.GPUID)
				acc := accumulators[key]
				if acc == nil {
					acc = &gpuStatsAccumulator{
						DeviceID: sample.DeviceID,
						GPUID:    gpu.GPUID,
						GPUName:  gpu.Name,
					}
					accumulators[key] = acc
				}
				acc.add(sample.Timestamp, gpu)
			}
		}); err != nil {
			return nil, err
		}
	}

	out := make([]GPUStats, 0, len(accumulators))
	for _, acc := range accumulators {
		out = append(out, acc.finish())
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DeviceID == out[j].DeviceID {
			return out[i].GPUID < out[j].GPUID
		}
		return out[i].DeviceID < out[j].DeviceID
	})
	return out, nil
}

func (s *MetricsStore) DiskStatus() (DiskStatus, error) {
	free, err := disk.FreeBytes(filepath.Dir(s.dir))
	if err != nil {
		return DiskStatus{}, err
	}
	status := "ok"
	if free < s.minFree {
		status = "critical"
	} else if free < s.minFree+256*1024*1024 {
		status = "warning"
	}
	return DiskStatus{
		FreeBytes:    free,
		MinFreeBytes: s.minFree,
		Status:       status,
	}, nil
}

func (s *MetricsStore) ensureWritable() error {
	status, err := s.DiskStatus()
	if err != nil {
		return err
	}
	if status.FreeBytes < status.MinFreeBytes {
		return ErrInsufficientStorage
	}
	return nil
}

func (s *MetricsStore) appendSegmentLocked(segment string, samples []StoredSample) error {
	path := filepath.Join(s.dir, "samples-"+segment+".jsonl.gz")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	enc := json.NewEncoder(gw)
	for _, sample := range samples {
		if err := enc.Encode(sample); err != nil {
			_ = gw.Close()
			return err
		}
	}
	return gw.Close()
}

func (s *MetricsStore) loadLatest() error {
	files, err := s.segmentFiles()
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-s.retention)
	for _, path := range files {
		if !segmentMayOverlap(path, cutoff) {
			continue
		}
		if err := scanSegment(path, func(sample StoredSample) {
			if sample.Timestamp.Before(cutoff) {
				return
			}
			for _, gpu := range sample.GPUs {
				key := latestKey(sample.DeviceID, gpu.GPUID)
				current, ok := s.latest[key]
				if !ok || sample.Timestamp.After(current.Timestamp) {
					s.latest[key] = StoredGPU{DeviceID: sample.DeviceID, Timestamp: sample.Timestamp, GPU: gpu}
				}
			}
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *MetricsStore) cleanupLocked() error {
	if s.retention <= 0 {
		return nil
	}
	cutoff := time.Now().Add(-s.retention)
	files, err := s.segmentFiles()
	if err != nil {
		return err
	}
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}
	}
	return nil
}

func (s *MetricsStore) segmentFiles() ([]string, error) {
	pattern := filepath.Join(s.dir, "samples-*.jsonl.gz")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func scanSegment(path string, visit func(StoredSample)) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	gr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gr.Close()
	reader := bufio.NewReader(gr)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var sample StoredSample
			if json.Unmarshal(line, &sample) == nil {
				visit(sample)
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func segmentMayOverlap(path string, since time.Time) bool {
	base := filepath.Base(path)
	segment := strings.TrimSuffix(strings.TrimPrefix(base, "samples-"), ".jsonl.gz")
	at, err := time.Parse("2006010215", segment)
	if err != nil {
		return true
	}
	return at.Add(time.Hour).After(since)
}

func latestKey(deviceID, gpuID string) string {
	return fmt.Sprintf("%s/%s", deviceID, gpuID)
}

type DiskStatus struct {
	FreeBytes    uint64 `json:"free_bytes"`
	MinFreeBytes uint64 `json:"min_free_bytes"`
	Status       string `json:"status"`
}

type gpuStatsAccumulator struct {
	DeviceID  string
	GPUID     string
	GPUName   string
	Count     int
	First     time.Time
	Last      time.Time
	UtilCount int
	UtilSum   float64
	UtilPeak  *float64
	IdleCount int
	MemSum    uint64
	MemPeak   uint64
	MemTotal  uint64
	TempPeak  *float64
	PowerPeak *float64
}

func (a *gpuStatsAccumulator) add(timestamp time.Time, gpu model.GPUStatus) {
	a.Count++
	if a.First.IsZero() || timestamp.Before(a.First) {
		a.First = timestamp
	}
	if a.Last.IsZero() || timestamp.After(a.Last) {
		a.Last = timestamp
	}
	if gpu.Name != "" {
		a.GPUName = gpu.Name
	}
	if gpu.MemoryTotalBytes > 0 {
		a.MemTotal = gpu.MemoryTotalBytes
	}
	a.MemSum += gpu.MemoryUsedBytes
	if gpu.MemoryUsedBytes > a.MemPeak {
		a.MemPeak = gpu.MemoryUsedBytes
	}
	if gpu.UtilizationGPUPercent != nil {
		util := *gpu.UtilizationGPUPercent
		a.UtilCount++
		a.UtilSum += util
		if util < 5 {
			a.IdleCount++
		}
		if a.UtilPeak == nil || util > *a.UtilPeak {
			value := util
			a.UtilPeak = &value
		}
	}
	if gpu.TemperatureCelsius != nil {
		value := *gpu.TemperatureCelsius
		if a.TempPeak == nil || value > *a.TempPeak {
			a.TempPeak = &value
		}
	}
	if gpu.PowerDrawWatts != nil {
		value := *gpu.PowerDrawWatts
		if a.PowerPeak == nil || value > *a.PowerPeak {
			a.PowerPeak = &value
		}
	}
}

func (a *gpuStatsAccumulator) finish() GPUStats {
	var avgUtil *float64
	if a.UtilCount > 0 {
		value := a.UtilSum / float64(a.UtilCount)
		avgUtil = &value
	}
	avgMem := uint64(0)
	if a.Count > 0 {
		avgMem = a.MemSum / uint64(a.Count)
	}
	idlePercent := 0.0
	if a.UtilCount > 0 {
		idlePercent = float64(a.IdleCount) / float64(a.UtilCount) * 100
	}
	return GPUStats{
		DeviceID:                  a.DeviceID,
		GPUID:                     a.GPUID,
		GPUName:                   a.GPUName,
		SampleCount:               a.Count,
		FirstSampleAt:             a.First,
		LastSampleAt:              a.Last,
		AverageUtilizationPercent: avgUtil,
		PeakUtilizationPercent:    a.UtilPeak,
		IdleSampleCount:           a.IdleCount,
		IdleSamplePercent:         idlePercent,
		AverageMemoryUsedBytes:    avgMem,
		PeakMemoryUsedBytes:       a.MemPeak,
		MemoryTotalBytes:          a.MemTotal,
		PeakTemperatureCelsius:    a.TempPeak,
		PeakPowerDrawWatts:        a.PowerPeak,
	}
}
