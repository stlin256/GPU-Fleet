package agent

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gpufleet/internal/model"
)

func (r Runner) ConfigReport(ctx context.Context, sample model.GPUSample) model.AgentConfigReport {
	report := model.AgentConfigReport{
		DeviceID:              r.Client.DeviceID,
		AgentVersion:          model.AgentVersion,
		CollectedAt:           time.Now().UTC(),
		Hostname:              hostname(),
		OS:                    runtime.GOOS,
		OSVersion:             runtime.GOARCH,
		Architecture:          runtime.GOARCH,
		Runtime:               runtime.Version(),
		ServerURL:             r.Client.ServerURL,
		NvidiaSMICommand:      r.Collector.Command,
		NvidiaSMIResolvedPath: resolveCommandPath(r.Collector.Command),
		NvidiaSMIVersion:      r.Collector.Version(ctx),
		SampleIntervalSeconds: int(r.SampleInterval.Seconds()),
		ConfigIntervalSeconds: int(r.ConfigInterval.Seconds()),
		ProcessesEnabled:      r.CollectProcesses,
		GzipEnabled:           r.Client.UseGzip,
		QueuePath:             r.Queue.Path(),
		QueueMaxBytes:         r.Queue.MaxBytes(),
	}
	if path, err := os.Executable(); err == nil {
		report.ExecutablePath = path
	}
	if wd, err := os.Getwd(); err == nil {
		report.WorkingDirectory = wd
	}
	for _, gpu := range sample.GPUs {
		report.GPUs = append(report.GPUs, model.AgentGPUConfig{
			GPUID:                 gpu.GPUID,
			UUIDHash:              gpu.UUIDHash,
			Name:                  gpu.Name,
			DriverVersion:         gpu.DriverVersion,
			VBIOSVersion:          gpu.VBIOSVersion,
			MemoryTotalBytes:      gpu.MemoryTotalBytes,
			MemoryFreeBytes:       gpu.MemoryFreeBytes,
			MemoryReservedBytes:   gpu.MemoryReservedBytes,
			TemperatureLimitC:     gpu.TemperatureLimitC,
			PowerLimitWatts:       gpu.PowerLimitWatts,
			PowerEnforcedLimitW:   gpu.PowerEnforcedLimitW,
			PCIeLinkGeneration:    gpu.PCIeLinkGeneration,
			PCIeLinkGenerationMax: gpu.PCIeLinkGenerationMax,
			PCIeLinkWidth:         gpu.PCIeLinkWidth,
			PCIeLinkWidthMax:      gpu.PCIeLinkWidthMax,
			ComputeMode:           gpu.ComputeMode,
			ComputeCapability:     gpu.ComputeCapability,
			DisplayActive:         gpu.DisplayActive,
			DisplayAttached:       gpu.DisplayAttached,
			PersistenceMode:       gpu.PersistenceMode,
			DriverModel:           gpu.DriverModel,
			ECCModeCurrent:        gpu.ECCModeCurrent,
			MIGModeCurrent:        gpu.MIGModeCurrent,
			ClockThrottleReasons:  gpu.ClockThrottleReasons,
			CollectionError:       gpu.CollectionError,
		})
		if gpu.CollectionError != "" {
			report.CollectionErrors = append(report.CollectionErrors, gpu.CollectionError)
		}
	}
	return report
}

func resolveCommandPath(command string) string {
	if strings.TrimSpace(command) == "" {
		command = "nvidia-smi"
	}
	if path, err := exec.LookPath(command); err == nil {
		return path
	}
	return ""
}

func (c Collector) Version(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Command, "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return limitConfigText(string(bytes.TrimSpace(output)), 2000)
}

func limitConfigText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}
