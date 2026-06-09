package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gpufleet/internal/model"
)

func TestConfigReportContainsRuntimeAndNoSecret(t *testing.T) {
	queue, err := NewSampleQueue(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{
		Client: &Client{
			ServerURL: "https://agent:token@example.com:9008",
			DeviceID:  "device-test",
			Secret:    "super-secret",
			UseGzip:   true,
		},
		Collector:        NewCollector("nvidia-smi", time.Second),
		Queue:            queue,
		SampleInterval:   10 * time.Second,
		ConfigInterval:   time.Hour,
		CollectProcesses: true,
	}
	sample := model.GPUSample{
		Timestamp: time.Now().UTC(),
		GPUs: []model.GPUStatus{{
			GPUID:            "gpu0",
			Name:             "Tesla V100",
			UUIDHash:         "sha256:test",
			DriverVersion:    "550.01",
			MemoryTotalBytes: 16 * 1024 * 1024 * 1024,
		}},
	}
	report := runner.ConfigReport(context.Background(), sample)
	if report.DeviceID != "device-test" || report.AgentVersion == "" || report.Runtime == "" {
		t.Fatalf("unexpected report identity/runtime fields: %+v", report)
	}
	if report.ServerURL != runner.Client.ServerURL || report.QueuePath == "" || report.QueueMaxBytes == 0 {
		t.Fatalf("unexpected report runtime config: %+v", report)
	}
	if len(report.GPUs) != 1 || report.GPUs[0].Name != "Tesla V100" {
		t.Fatalf("expected GPU config to be copied from sample, got %+v", report.GPUs)
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), runner.Client.Secret) {
		t.Fatal("config report must not include the device secret")
	}
}
