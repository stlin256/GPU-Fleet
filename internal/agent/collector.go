package agent

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"gpufleet/internal/auth"
	"gpufleet/internal/model"
)

type Collector struct {
	Command string
	Timeout time.Duration
}

func NewCollector(command string, timeout time.Duration) Collector {
	if command == "" {
		command = "nvidia-smi"
	}
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return Collector{Command: command, Timeout: timeout}
}

func (c Collector) Collect(ctx context.Context) (model.GPUSample, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	args := []string{
		"--query-gpu=index,name,uuid,driver_version,memory.total,memory.used,utilization.gpu,temperature.gpu,power.draw,fan.speed,clocks.gr,clocks.mem,pstate,pcie.link.gen.current,pcie.link.width.current",
		"--format=csv,noheader,nounits",
	}
	cmd := exec.CommandContext(ctx, c.Command, args...)
	output, err := cmd.Output()
	if err != nil {
		return model.GPUSample{
			Timestamp: time.Now().UTC(),
			GPUs: []model.GPUStatus{{
				GPUID:           "gpu0",
				CollectionError: collectionError(err, ctx.Err()),
			}},
		}, err
	}

	reader := csv.NewReader(bytes.NewReader(output))
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return model.GPUSample{}, err
	}

	sample := model.GPUSample{Timestamp: time.Now().UTC()}
	for _, record := range records {
		if len(record) < 15 {
			continue
		}
		totalMiB := parseUint(record[4])
		usedMiB := parseUint(record[5])
		status := model.GPUStatus{
			GPUID:              "gpu" + clean(record[0]),
			Name:               clean(record[1]),
			UUIDHash:           "sha256:" + auth.SHA256Hex(clean(record[2])),
			DriverVersion:      clean(record[3]),
			MemoryTotalBytes:   totalMiB * 1024 * 1024,
			MemoryUsedBytes:    usedMiB * 1024 * 1024,
			PCIeLinkGeneration: clean(record[13]),
			PCIeLinkWidth:      clean(record[14]),
			PState:             clean(record[12]),
		}
		status.UtilizationGPUPercent = parseFloatPtr(record[6])
		status.TemperatureCelsius = parseFloatPtr(record[7])
		status.PowerDrawWatts = parseFloatPtr(record[8])
		status.FanSpeedPercent = parseFloatPtr(record[9])
		status.GraphicsClockMHz = parseFloatPtr(record[10])
		status.MemoryClockMHz = parseFloatPtr(record[11])
		sample.GPUs = append(sample.GPUs, status)
	}
	return sample, nil
}

func clean(value string) string {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "[not supported]") ||
		strings.EqualFold(value, "not supported") ||
		strings.EqualFold(value, "N/A") ||
		strings.EqualFold(value, "[N/A]") {
		return ""
	}
	return value
}

func parseFloatPtr(value string) *float64 {
	cleaned := clean(value)
	if cleaned == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseUint(value string) uint64 {
	cleaned := clean(value)
	if cleaned == "" {
		return 0
	}
	parsed, err := strconv.ParseUint(cleaned, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func collectionError(err error, ctxErr error) string {
	if ctxErr != nil {
		return "collection_timeout"
	}
	if err == nil {
		return ""
	}
	return fmt.Sprintf("nvidia_smi_failed: %v", err)
}
