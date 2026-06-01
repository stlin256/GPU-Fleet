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
	Client         *Client
	Collector      Collector
	SampleInterval time.Duration
	Once           bool
}

func (r Runner) Run(ctx context.Context) error {
	if r.Client == nil {
		return fmt.Errorf("client is required")
	}
	if r.SampleInterval == 0 {
		r.SampleInterval = 10 * time.Second
	}
	if r.Collector.Command == "" {
		r.Collector = NewCollector("", 5*time.Second)
	}

	if err := r.collectAndUpload(ctx); err != nil {
		return err
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
			if err := r.collectAndUpload(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "upload failed: %v\n", err)
			}
		}
	}
}

func (r Runner) collectAndUpload(ctx context.Context) error {
	sample, err := r.Collector.Collect(ctx)
	heartbeat := model.Heartbeat{
		AgentVersion: model.AgentVersion,
		Hostname:     hostname(),
		OS:           runtime.GOOS,
		OSVersion:    runtime.GOARCH,
		GPUCount:     len(sample.GPUs),
		Timestamp:    time.Now().UTC(),
	}
	if hbErr := r.Client.PostHeartbeat(heartbeat); hbErr != nil {
		return hbErr
	}
	batch := model.SampleBatch{
		DeviceID:     r.Client.DeviceID,
		AgentVersion: model.AgentVersion,
		Samples:      []model.GPUSample{sample},
	}
	if postErr := r.Client.PostSamples(batch); postErr != nil {
		return postErr
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
