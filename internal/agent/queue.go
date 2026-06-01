package agent

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"gpufleet/internal/model"
)

type SampleQueue struct {
	path     string
	maxBytes int64
	mu       sync.Mutex
}

func NewSampleQueue(dir string, maxBytes int64) (*SampleQueue, error) {
	if dir == "" || maxBytes <= 0 {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &SampleQueue{
		path:     filepath.Join(dir, "samples.jsonl"),
		maxBytes: maxBytes,
	}, nil
}

func (q *SampleQueue) Enqueue(batch model.SampleBatch) error {
	if q == nil || len(batch.Samples) == 0 {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	file, err := os.OpenFile(q.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(file).Encode(batch); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return q.trimLocked()
}

func (q *SampleQueue) Flush(client *Client, maxBatches int) error {
	if q == nil || client == nil {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	batches, err := q.readLocked()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(batches) == 0 {
		return nil
	}
	if maxBatches <= 0 || maxBatches > len(batches) {
		maxBatches = len(batches)
	}

	sent := 0
	for sent < maxBatches {
		if err := client.PostSamples(batches[sent]); err != nil {
			return q.rewriteLocked(batches[sent:])
		}
		sent++
	}
	return q.rewriteLocked(batches[sent:])
}

func (q *SampleQueue) readLocked() ([]model.SampleBatch, error) {
	file, err := os.Open(q.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var batches []model.SampleBatch
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var batch model.SampleBatch
		if err := json.Unmarshal(scanner.Bytes(), &batch); err == nil && len(batch.Samples) > 0 {
			batches = append(batches, batch)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return batches, nil
}

func (q *SampleQueue) rewriteLocked(batches []model.SampleBatch) error {
	if len(batches) == 0 {
		if err := os.Remove(q.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	tmp := q.path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(file)
	for _, batch := range batches {
		if err := enc.Encode(batch); err != nil {
			_ = file.Close()
			return err
		}
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, q.path); err != nil {
		return err
	}
	return q.trimLocked()
}

func (q *SampleQueue) trimLocked() error {
	info, err := os.Stat(q.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Size() <= q.maxBytes {
		return nil
	}
	batches, err := q.readLocked()
	if err != nil {
		return err
	}
	for len(batches) > 0 {
		batches = batches[1:]
		if estimateQueueBytes(batches) <= q.maxBytes {
			break
		}
	}
	return q.rewriteLocked(batches)
}

func estimateQueueBytes(batches []model.SampleBatch) int64 {
	var total int64
	for _, batch := range batches {
		raw, err := json.Marshal(batch)
		if err == nil {
			total += int64(len(raw) + 1)
		}
	}
	return total
}
