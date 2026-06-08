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

const (
	coldSegmentAge        = 7 * 24 * time.Hour
	compactSegmentComment = "gpufleet-compact-v1"
	rawIndexAge           = time.Hour
	minuteRollupAge       = 24 * time.Hour
	hourRollupAge         = 30 * 24 * time.Hour
)

type MetricsStore struct {
	dir            string
	minFree        uint64
	retention      time.Duration
	mu             sync.RWMutex
	writeMu        sync.Mutex
	segmentLocksMu sync.Mutex
	segmentLocks   map[string]*sync.RWMutex
	latest         map[string]StoredGPU
	rawIndex       map[string][]SeriesPoint
	minuteRollups  map[string]map[int64]*metricRollupBucket
	hourRollups    map[string]map[int64]*metricRollupBucket
	indexReady     bool
	lastCleanup    time.Time
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
		dir:           dir,
		minFree:       minFreeBytes,
		retention:     retention,
		latest:        map[string]StoredGPU{},
		rawIndex:      map[string][]SeriesPoint{},
		minuteRollups: map[string]map[int64]*metricRollupBucket{},
		hourRollups:   map[string]map[int64]*metricRollupBucket{},
		segmentLocks:  map[string]*sync.RWMutex{},
	}
	if err := store.loadLatest(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *MetricsStore) AppendBatch(batch model.SampleBatch) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	s.mu.RLock()
	cleanupDue := time.Since(s.lastCleanup) > time.Hour
	s.mu.RUnlock()
	if cleanupDue {
		if err := s.cleanup(); err != nil {
			return err
		}
		s.mu.Lock()
		s.lastCleanup = time.Now()
		s.mu.Unlock()
	}
	if err := s.ensureWritable(); err != nil {
		return err
	}

	bySegment := map[string][]StoredSample{}
	latestUpdates := map[string]StoredGPU{}
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
			current, ok := latestUpdates[key]
			if !ok || stored.Timestamp.After(current.Timestamp) {
				latestUpdates[key] = StoredGPU{DeviceID: batch.DeviceID, Timestamp: stored.Timestamp, GPU: gpu}
			}
		}
	}

	for segment, samples := range bySegment {
		if err := s.appendSegmentLocked(segment, samples); err != nil {
			return err
		}
	}
	s.mu.Lock()
	for key, next := range latestUpdates {
		current, ok := s.latest[key]
		if !ok || next.Timestamp.After(current.Timestamp) {
			s.latest[key] = next
		}
	}
	for _, samples := range bySegment {
		for _, sample := range samples {
			s.indexSampleLocked(sample)
		}
	}
	s.pruneIndexLocked(time.Now().UTC())
	s.indexReady = true
	s.mu.Unlock()
	return nil
}

func (s *MetricsStore) Latest() []StoredGPU {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
	if points, ok := s.seriesFromIndex(deviceID, gpuID, since); ok {
		return points, nil
	}
	files, err := s.segmentFiles()
	if err != nil {
		return nil, err
	}
	points := []SeriesPoint{}
	for _, path := range files {
		if !segmentMayOverlap(path, since) {
			continue
		}
		if err := s.scanSegment(path, func(sample StoredSample) {
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
		}); err != nil && !isIgnorableSegmentScanError(err) {
			return nil, err
		}
	}
	sortSeriesPoints(points)
	return points, nil
}

func (s *MetricsStore) Stats(deviceID string, since time.Time) ([]GPUStats, error) {
	if stats, ok := s.statsFromIndex(deviceID, since); ok {
		return stats, nil
	}
	files, err := s.segmentFiles()
	if err != nil {
		return nil, err
	}
	accumulators := map[string]*gpuStatsAccumulator{}
	for _, path := range files {
		if !segmentMayOverlap(path, since) {
			continue
		}
		if err := s.scanSegment(path, func(sample StoredSample) {
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
		}); err != nil && !isIgnorableSegmentScanError(err) {
			return nil, err
		}
	}

	return finishStats(accumulators), nil
}

func (s *MetricsStore) DiskStatus() (DiskStatus, error) {
	free, err := disk.FreeBytes(filepath.Dir(s.dir))
	if err != nil {
		return DiskStatus{}, err
	}
	s.mu.RLock()
	minFree := s.minFree
	s.mu.RUnlock()
	status := "ok"
	if free < minFree {
		status = "critical"
	} else if free < minFree+256*1024*1024 {
		status = "warning"
	}
	return DiskStatus{
		FreeBytes:    free,
		MinFreeBytes: minFree,
		Status:       status,
	}, nil
}

func (s *MetricsStore) SetMinFreeBytes(minFreeBytes uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.minFree = minFreeBytes
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

func (s *MetricsStore) StoredDays() int {
	files, err := s.segmentFiles()
	if err != nil {
		return 0
	}
	var first time.Time
	var last time.Time
	for _, path := range files {
		at, ok := segmentStart(path)
		if !ok {
			continue
		}
		if first.IsZero() || at.Before(first) {
			first = at
		}
		end := at.Add(time.Hour)
		if last.IsZero() || end.After(last) {
			last = end
		}
	}
	if first.IsZero() || last.IsZero() || !last.After(first) {
		return 0
	}
	days := int(last.Sub(first).Hours()+23) / 24
	if days < 1 {
		return 1
	}
	return days
}

func (s *MetricsStore) indexSampleLocked(sample StoredSample) {
	for _, gpu := range sample.GPUs {
		key := latestKey(sample.DeviceID, gpu.GPUID)
		point := SeriesPoint{
			Timestamp:             sample.Timestamp,
			UtilizationGPUPercent: gpu.UtilizationGPUPercent,
			MemoryUsedBytes:       gpu.MemoryUsedBytes,
			MemoryTotalBytes:      gpu.MemoryTotalBytes,
			TemperatureCelsius:    gpu.TemperatureCelsius,
			PowerDrawWatts:        gpu.PowerDrawWatts,
		}
		s.rawIndex[key] = append(s.rawIndex[key], point)
		minuteBucket := sample.Timestamp.UTC().Truncate(time.Minute).Unix()
		hourBucket := sample.Timestamp.UTC().Truncate(time.Hour).Unix()
		s.rollupForLocked(s.minuteRollups, key, minuteBucket, sample.DeviceID, gpu.GPUID).add(sample.Timestamp, gpu)
		s.rollupForLocked(s.hourRollups, key, hourBucket, sample.DeviceID, gpu.GPUID).add(sample.Timestamp, gpu)
	}
}

func (s *MetricsStore) rollupForLocked(index map[string]map[int64]*metricRollupBucket, key string, bucketAt int64, deviceID, gpuID string) *metricRollupBucket {
	buckets := index[key]
	if buckets == nil {
		buckets = map[int64]*metricRollupBucket{}
		index[key] = buckets
	}
	bucket := buckets[bucketAt]
	if bucket == nil {
		bucket = &metricRollupBucket{
			DeviceID:  deviceID,
			GPUID:     gpuID,
			Timestamp: time.Unix(bucketAt, 0).UTC(),
		}
		buckets[bucketAt] = bucket
	}
	return bucket
}

func (s *MetricsStore) pruneIndexLocked(now time.Time) {
	rawCutoff := now.Add(-rawIndexAge)
	for key, points := range s.rawIndex {
		kept := points[:0]
		for _, point := range points {
			if !point.Timestamp.Before(rawCutoff) {
				kept = append(kept, point)
			}
		}
		if len(kept) == 0 {
			delete(s.rawIndex, key)
		} else {
			s.rawIndex[key] = kept
		}
	}
	pruneRollups(s.minuteRollups, now.Add(-minuteRollupAge))
	pruneRollups(s.hourRollups, now.Add(-hourRollupAge))
}

func pruneRollups(index map[string]map[int64]*metricRollupBucket, cutoff time.Time) {
	cutoffUnix := cutoff.Unix()
	for key, buckets := range index {
		for bucketAt := range buckets {
			if bucketAt < cutoffUnix {
				delete(buckets, bucketAt)
			}
		}
		if len(buckets) == 0 {
			delete(index, key)
		}
	}
}

func (s *MetricsStore) seriesFromIndex(deviceID, gpuID string, since time.Time) ([]SeriesPoint, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.indexReady {
		return nil, false
	}
	now := time.Now().UTC()
	key := latestKey(deviceID, gpuID)
	if !since.Before(now.Add(-rawIndexAge)) {
		points := make([]SeriesPoint, 0, len(s.rawIndex[key]))
		for _, point := range s.rawIndex[key] {
			if !point.Timestamp.Before(since) {
				points = append(points, point)
			}
		}
		sortSeriesPoints(points)
		return points, true
	}
	return nil, false
}

func seriesFromRollups(buckets map[int64]*metricRollupBucket, since time.Time) []SeriesPoint {
	points := make([]SeriesPoint, 0, len(buckets))
	for _, bucket := range buckets {
		if bucket.Last.Before(since) {
			continue
		}
		points = append(points, bucket.seriesPoint())
	}
	sortSeriesPoints(points)
	return points
}

func (s *MetricsStore) statsFromIndex(deviceID string, since time.Time) ([]GPUStats, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.indexReady {
		return nil, false
	}
	now := time.Now().UTC()
	var source map[string]map[int64]*metricRollupBucket
	if !since.Before(now.Add(-minuteRollupAge)) {
		source = s.minuteRollups
	} else if !since.Before(now.Add(-hourRollupAge)) {
		source = s.hourRollups
	} else {
		return nil, false
	}
	accumulators := map[string]*gpuStatsAccumulator{}
	for key, buckets := range source {
		for _, bucket := range buckets {
			if bucket.Last.Before(since) {
				continue
			}
			if deviceID != "" && bucket.DeviceID != deviceID {
				continue
			}
			acc := accumulators[key]
			if acc == nil {
				acc = &gpuStatsAccumulator{
					DeviceID: bucket.DeviceID,
					GPUID:    bucket.GPUID,
					GPUName:  bucket.GPUName,
				}
				accumulators[key] = acc
			}
			acc.addBucket(bucket)
		}
	}
	return finishStats(accumulators), true
}

func (s *MetricsStore) appendSegmentLocked(segment string, samples []StoredSample) error {
	lock := s.segmentLockForName(segment)
	lock.Lock()
	defer lock.Unlock()

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
	indexCutoff := time.Now().Add(-hourRollupAge)
	if s.retention > 0 && cutoff.After(indexCutoff) {
		indexCutoff = cutoff
	}
	for _, path := range files {
		if !segmentMayOverlap(path, minTime(cutoff, indexCutoff)) {
			continue
		}
		if err := scanSegment(path, func(sample StoredSample) {
			if sample.Timestamp.Before(cutoff) {
				if sample.Timestamp.Before(indexCutoff) {
					return
				}
			}
			if !sample.Timestamp.Before(indexCutoff) {
				s.indexSampleLocked(sample)
			}
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
	s.pruneIndexLocked(time.Now().UTC())
	s.indexReady = true
	return nil
}

func (s *MetricsStore) cleanup() error {
	if s.retention <= 0 {
		return nil
	}
	cutoff := time.Now().Add(-s.retention)
	compactBefore := time.Now().Add(-coldSegmentAge)
	files, err := s.segmentFiles()
	if err != nil {
		return err
	}
	for _, path := range files {
		at, ok := segmentStart(path)
		if !ok {
			continue
		}
		if !at.Add(time.Hour).After(cutoff) {
			lock := s.segmentLockForName(at.Format("2006010215"))
			lock.Lock()
			_ = os.Remove(path)
			lock.Unlock()
			continue
		}
		if at.Add(time.Hour).Before(compactBefore) {
			lock := s.segmentLockForName(at.Format("2006010215"))
			lock.Lock()
			err := compactSegment(path, at)
			lock.Unlock()
			if err != nil {
				return err
			}
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

func (s *MetricsStore) scanSegment(path string, visit func(StoredSample)) error {
	lock := s.segmentLockForPath(path)
	lock.RLock()
	defer lock.RUnlock()
	return scanSegment(path, visit)
}

func (s *MetricsStore) segmentLockForPath(path string) *sync.RWMutex {
	if at, ok := segmentStart(path); ok {
		return s.segmentLockForName(at.Format("2006010215"))
	}
	return s.segmentLockForName(filepath.Base(path))
}

func (s *MetricsStore) segmentLockForName(segment string) *sync.RWMutex {
	s.segmentLocksMu.Lock()
	defer s.segmentLocksMu.Unlock()
	lock := s.segmentLocks[segment]
	if lock == nil {
		lock = &sync.RWMutex{}
		s.segmentLocks[segment] = lock
	}
	return lock
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

func isIgnorableSegmentScanError(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, io.ErrUnexpectedEOF)
}

func compactSegment(path string, segmentAt time.Time) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if isCompactedSegment(path, segmentAt, info) {
		return nil
	}

	tmp := path + ".tmp"
	_ = os.Remove(tmp)
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	gw, err := gzip.NewWriterLevel(file, gzip.BestCompression)
	if err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return err
	}
	gw.Name = filepath.Base(path)
	gw.Comment = compactSegmentComment
	gw.ModTime = segmentAt
	enc := json.NewEncoder(gw)
	var encodeErr error
	err = scanSegment(path, func(sample StoredSample) {
		if encodeErr != nil {
			return
		}
		if err := enc.Encode(sample); err != nil {
			encodeErr = err
		}
	})
	closeErr := gw.Close()
	fileErr := file.Close()
	if err != nil || encodeErr != nil || closeErr != nil || fileErr != nil {
		_ = os.Remove(tmp)
		if err != nil {
			return err
		}
		if encodeErr != nil {
			return encodeErr
		}
		if closeErr != nil {
			return closeErr
		}
		return fileErr
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chtimes(path, segmentAt, segmentAt)
}

func isCompactedSegment(path string, segmentAt time.Time, info os.FileInfo) bool {
	if info.ModTime().After(segmentAt.Add(time.Hour)) {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	gr, err := gzip.NewReader(file)
	if err != nil {
		return false
	}
	defer gr.Close()
	return gr.Header.Comment == compactSegmentComment
}

func segmentMayOverlap(path string, since time.Time) bool {
	at, ok := segmentStart(path)
	if !ok {
		return true
	}
	return at.Add(time.Hour).After(since)
}

func segmentStart(path string) (time.Time, bool) {
	base := filepath.Base(path)
	segment := strings.TrimSuffix(strings.TrimPrefix(base, "samples-"), ".jsonl.gz")
	at, err := time.Parse("2006010215", segment)
	return at, err == nil
}

func latestKey(deviceID, gpuID string) string {
	return fmt.Sprintf("%s/%s", deviceID, gpuID)
}

func sortSeriesPoints(points []SeriesPoint) {
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})
}

func finishStats(accumulators map[string]*gpuStatsAccumulator) []GPUStats {
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
	return out
}

func minTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}

type DiskStatus struct {
	FreeBytes    uint64 `json:"free_bytes"`
	MinFreeBytes uint64 `json:"min_free_bytes"`
	Status       string `json:"status"`
}

type metricRollupBucket struct {
	DeviceID   string
	GPUID      string
	GPUName    string
	Timestamp  time.Time
	Count      int
	First      time.Time
	Last       time.Time
	UtilCount  int
	UtilSum    float64
	UtilPeak   *float64
	IdleCount  int
	MemSum     uint64
	MemPeak    uint64
	MemTotal   uint64
	TempCount  int
	TempSum    float64
	TempPeak   *float64
	PowerCount int
	PowerSum   float64
	PowerPeak  *float64
}

func (b *metricRollupBucket) add(timestamp time.Time, gpu model.GPUStatus) {
	b.Count++
	if b.First.IsZero() || timestamp.Before(b.First) {
		b.First = timestamp
	}
	if b.Last.IsZero() || timestamp.After(b.Last) {
		b.Last = timestamp
	}
	if gpu.Name != "" {
		b.GPUName = gpu.Name
	}
	if gpu.MemoryTotalBytes > 0 {
		b.MemTotal = gpu.MemoryTotalBytes
	}
	b.MemSum += gpu.MemoryUsedBytes
	if gpu.MemoryUsedBytes > b.MemPeak {
		b.MemPeak = gpu.MemoryUsedBytes
	}
	if gpu.UtilizationGPUPercent != nil {
		util := *gpu.UtilizationGPUPercent
		b.UtilCount++
		b.UtilSum += util
		if util < 5 {
			b.IdleCount++
		}
		if b.UtilPeak == nil || util > *b.UtilPeak {
			value := util
			b.UtilPeak = &value
		}
	}
	if gpu.TemperatureCelsius != nil {
		temp := *gpu.TemperatureCelsius
		b.TempCount++
		b.TempSum += temp
		if b.TempPeak == nil || temp > *b.TempPeak {
			value := temp
			b.TempPeak = &value
		}
	}
	if gpu.PowerDrawWatts != nil {
		power := *gpu.PowerDrawWatts
		b.PowerCount++
		b.PowerSum += power
		if b.PowerPeak == nil || power > *b.PowerPeak {
			value := power
			b.PowerPeak = &value
		}
	}
}

func (b *metricRollupBucket) seriesPoint() SeriesPoint {
	point := SeriesPoint{
		Timestamp:        b.Timestamp,
		MemoryTotalBytes: b.MemTotal,
	}
	if b.Count > 0 {
		point.MemoryUsedBytes = b.MemSum / uint64(b.Count)
	}
	if b.UtilCount > 0 {
		value := b.UtilSum / float64(b.UtilCount)
		point.UtilizationGPUPercent = &value
	}
	if b.TempCount > 0 {
		value := b.TempSum / float64(b.TempCount)
		point.TemperatureCelsius = &value
	}
	if b.PowerCount > 0 {
		value := b.PowerSum / float64(b.PowerCount)
		point.PowerDrawWatts = &value
	}
	return point
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

func (a *gpuStatsAccumulator) addBucket(bucket *metricRollupBucket) {
	a.Count += bucket.Count
	if a.First.IsZero() || (!bucket.First.IsZero() && bucket.First.Before(a.First)) {
		a.First = bucket.First
	}
	if a.Last.IsZero() || bucket.Last.After(a.Last) {
		a.Last = bucket.Last
	}
	if bucket.GPUName != "" {
		a.GPUName = bucket.GPUName
	}
	if bucket.MemTotal > 0 {
		a.MemTotal = bucket.MemTotal
	}
	a.MemSum += bucket.MemSum
	if bucket.MemPeak > a.MemPeak {
		a.MemPeak = bucket.MemPeak
	}
	a.UtilCount += bucket.UtilCount
	a.UtilSum += bucket.UtilSum
	a.IdleCount += bucket.IdleCount
	if bucket.UtilPeak != nil && (a.UtilPeak == nil || *bucket.UtilPeak > *a.UtilPeak) {
		value := *bucket.UtilPeak
		a.UtilPeak = &value
	}
	if bucket.TempPeak != nil && (a.TempPeak == nil || *bucket.TempPeak > *a.TempPeak) {
		value := *bucket.TempPeak
		a.TempPeak = &value
	}
	if bucket.PowerPeak != nil && (a.PowerPeak == nil || *bucket.PowerPeak > *a.PowerPeak) {
		value := *bucket.PowerPeak
		a.PowerPeak = &value
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
