package model

import "time"

const AgentVersion = "0.1.0"

type Heartbeat struct {
	AgentVersion string    `json:"agent_version"`
	Hostname     string    `json:"hostname"`
	OS           string    `json:"os"`
	OSVersion    string    `json:"os_version,omitempty"`
	GPUCount     int       `json:"gpu_count"`
	Timestamp    time.Time `json:"timestamp"`
}

type SampleBatch struct {
	DeviceID     string      `json:"device_id"`
	AgentVersion string      `json:"agent_version"`
	Samples      []GPUSample `json:"samples"`
}

type GPUSample struct {
	Timestamp time.Time   `json:"timestamp"`
	GPUs      []GPUStatus `json:"gpus"`
}

type GPUStatus struct {
	GPUID                 string   `json:"gpu_id"`
	UUIDHash              string   `json:"uuid_hash"`
	Name                  string   `json:"name"`
	DriverVersion         string   `json:"driver_version"`
	MemoryTotalBytes      uint64   `json:"memory_total_bytes"`
	MemoryUsedBytes       uint64   `json:"memory_used_bytes"`
	UtilizationGPUPercent *float64 `json:"utilization_gpu_percent,omitempty"`
	TemperatureCelsius    *float64 `json:"temperature_celsius,omitempty"`
	PowerDrawWatts        *float64 `json:"power_draw_watts,omitempty"`
	FanSpeedPercent       *float64 `json:"fan_speed_percent,omitempty"`
	GraphicsClockMHz      *float64 `json:"graphics_clock_mhz,omitempty"`
	MemoryClockMHz        *float64 `json:"memory_clock_mhz,omitempty"`
	PState                string   `json:"pstate,omitempty"`
	PCIeLinkGeneration    string   `json:"pcie_link_generation,omitempty"`
	PCIeLinkWidth         string   `json:"pcie_link_width,omitempty"`
	CollectionError       string   `json:"collection_error,omitempty"`
}

type ProcessBatch struct {
	DeviceID  string            `json:"device_id"`
	Timestamp time.Time         `json:"timestamp"`
	Processes []ProcessSnapshot `json:"processes"`
}

type ProcessSnapshot struct {
	GPUID           string  `json:"gpu_id"`
	UUIDHash        string  `json:"uuid_hash"`
	PID             int     `json:"pid"`
	ProcessName     string  `json:"process_name"`
	UsedMemoryBytes uint64  `json:"used_memory_bytes"`
	Username        *string `json:"username,omitempty"`
	CommandLine     *string `json:"commandline,omitempty"`
}

type APIError struct {
	Error string `json:"error"`
}
