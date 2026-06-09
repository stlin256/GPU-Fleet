package agent

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"gpufleet/internal/model"
)

type Runner struct {
	Client           *Client
	Collector        Collector
	Queue            *SampleQueue
	SampleInterval   time.Duration
	ConfigInterval   time.Duration
	CollectProcesses bool
	Once             bool
}

func (r Runner) Run(ctx context.Context) error {
	if r.Client == nil {
		return fmt.Errorf("client is required")
	}
	if r.SampleInterval == 0 {
		r.SampleInterval = 10 * time.Second
	}
	if r.ConfigInterval == 0 {
		r.ConfigInterval = time.Hour
	}
	if r.Collector.Command == "" {
		r.Collector = NewCollector("", 5*time.Second)
	}

	configUploadedAt := time.Time{}
	if err := r.collectAndUpload(ctx, true); err != nil {
		if r.Once {
			return err
		}
		fmt.Fprintf(os.Stderr, "upload failed: %v\n", err)
	} else {
		configUploadedAt = time.Now()
	}
	if r.Once {
		return nil
	}

	ticker := time.NewTicker(r.SampleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			includeConfig := r.ConfigInterval > 0 && time.Since(configUploadedAt) >= r.ConfigInterval
			if err := r.collectAndUpload(ctx, includeConfig); err != nil {
				fmt.Fprintf(os.Stderr, "upload failed: %v\n", err)
			} else if includeConfig {
				configUploadedAt = time.Now()
			}
		}
	}
}

func (r Runner) collectAndUpload(ctx context.Context, includeConfig bool) error {
	sample, err := r.Collector.Collect(ctx)
	batch := model.SampleBatch{
		DeviceID:     r.Client.DeviceID,
		AgentVersion: model.AgentVersion,
		Samples:      []model.GPUSample{sample},
	}
	heartbeat := model.Heartbeat{
		AgentVersion: model.AgentVersion,
		Hostname:     hostname(),
		OS:           runtime.GOOS,
		OSVersion:    runtime.GOARCH,
		GPUCount:     len(sample.GPUs),
		Timestamp:    time.Now().UTC(),
	}
	if hbErr := r.Client.PostHeartbeat(heartbeat); hbErr != nil {
		_ = r.Queue.Enqueue(batch)
		return hbErr
	}
	if r.Queue != nil {
		if flushErr := r.Queue.Flush(r.Client, 100); flushErr != nil {
			_ = r.Queue.Enqueue(batch)
			return flushErr
		}
	}
	if includeConfig {
		if configErr := r.Client.PostConfig(r.ConfigReport(ctx, sample)); configErr != nil {
			fmt.Fprintf(os.Stderr, "config upload failed: %v\n", configErr)
		}
	}
	if postErr := r.Client.PostSamples(batch); postErr != nil {
		_ = r.Queue.Enqueue(batch)
		return postErr
	}
	if r.CollectProcesses {
		processes, processErr := r.Collector.CollectProcesses(ctx, sample)
		processes.DeviceID = r.Client.DeviceID
		if processErr == nil {
			if postErr := r.Client.PostProcesses(processes); postErr != nil {
				return postErr
			}
		}
	}
	return err
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}
