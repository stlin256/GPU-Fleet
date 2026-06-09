package model

import (
	"time"

	"gpufleet/internal/version"
)

var AgentVersion = version.Version

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
	VBIOSVersion          string   `json:"vbios_version,omitempty"`
	MemoryTotalBytes      uint64   `json:"memory_total_bytes"`
	MemoryUsedBytes       uint64   `json:"memory_used_bytes"`
	MemoryFreeBytes       uint64   `json:"memory_free_bytes,omitempty"`
	MemoryReservedBytes   uint64   `json:"memory_reserved_bytes,omitempty"`
	UtilizationGPUPercent *float64 `json:"utilization_gpu_percent,omitempty"`
	UtilizationMemPercent *float64 `json:"utilization_memory_percent,omitempty"`
	TemperatureCelsius    *float64 `json:"temperature_celsius,omitempty"`
	TemperatureMemCelsius *float64 `json:"temperature_memory_celsius,omitempty"`
	TemperatureLimitC     *float64 `json:"temperature_limit_celsius,omitempty"`
	PowerDrawWatts        *float64 `json:"power_draw_watts,omitempty"`
	PowerLimitWatts       *float64 `json:"power_limit_watts,omitempty"`
	PowerEnforcedLimitW   *float64 `json:"power_enforced_limit_watts,omitempty"`
	FanSpeedPercent       *float64 `json:"fan_speed_percent,omitempty"`
	GraphicsClockMHz      *float64 `json:"graphics_clock_mhz,omitempty"`
	MemoryClockMHz        *float64 `json:"memory_clock_mhz,omitempty"`
	SMClockMHz            *float64 `json:"sm_clock_mhz,omitempty"`
	VideoClockMHz         *float64 `json:"video_clock_mhz,omitempty"`
	PState                string   `json:"pstate,omitempty"`
	PCIeLinkGeneration    string   `json:"pcie_link_generation,omitempty"`
	PCIeLinkWidth         string   `json:"pcie_link_width,omitempty"`
	PCIeLinkGenerationMax string   `json:"pcie_link_generation_max,omitempty"`
	PCIeLinkWidthMax      string   `json:"pcie_link_width_max,omitempty"`
	ComputeMode           string   `json:"compute_mode,omitempty"`
	ComputeCapability     string   `json:"compute_capability,omitempty"`
	DisplayActive         string   `json:"display_active,omitempty"`
	DisplayAttached       string   `json:"display_attached,omitempty"`
	PersistenceMode       string   `json:"persistence_mode,omitempty"`
	DriverModel           string   `json:"driver_model,omitempty"`
	ECCModeCurrent        string   `json:"ecc_mode_current,omitempty"`
	MIGModeCurrent        string   `json:"mig_mode_current,omitempty"`
	ClockThrottleReasons  string   `json:"clock_throttle_reasons,omitempty"`
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

type AgentConfigReport struct {
	DeviceID              string           `json:"device_id,omitempty"`
	AgentVersion          string           `json:"agent_version"`
	CollectedAt           time.Time        `json:"collected_at"`
	Hostname              string           `json:"hostname,omitempty"`
	OS                    string           `json:"os,omitempty"`
	OSVersion             string           `json:"os_version,omitempty"`
	Architecture          string           `json:"architecture,omitempty"`
	Runtime               string           `json:"runtime,omitempty"`
	ExecutablePath        string           `json:"executable_path,omitempty"`
	WorkingDirectory      string           `json:"working_directory,omitempty"`
	ServerURL             string           `json:"server_url,omitempty"`
	NvidiaSMICommand      string           `json:"nvidia_smi_command,omitempty"`
	NvidiaSMIResolvedPath string           `json:"nvidia_smi_resolved_path,omitempty"`
	NvidiaSMIVersion      string           `json:"nvidia_smi_version,omitempty"`
	SampleIntervalSeconds int              `json:"sample_interval_seconds,omitempty"`
	ConfigIntervalSeconds int              `json:"config_interval_seconds,omitempty"`
	ProcessesEnabled      bool             `json:"processes_enabled"`
	GzipEnabled           bool             `json:"gzip_enabled"`
	QueuePath             string           `json:"queue_path,omitempty"`
	QueueMaxBytes         int64            `json:"queue_max_bytes,omitempty"`
	GPUs                  []AgentGPUConfig `json:"gpus,omitempty"`
	CollectionErrors      []string         `json:"collection_errors,omitempty"`
}

type AgentGPUConfig struct {
	GPUID                 string   `json:"gpu_id"`
	UUIDHash              string   `json:"uuid_hash,omitempty"`
	Name                  string   `json:"name,omitempty"`
	DriverVersion         string   `json:"driver_version,omitempty"`
	VBIOSVersion          string   `json:"vbios_version,omitempty"`
	MemoryTotalBytes      uint64   `json:"memory_total_bytes,omitempty"`
	MemoryFreeBytes       uint64   `json:"memory_free_bytes,omitempty"`
	MemoryReservedBytes   uint64   `json:"memory_reserved_bytes,omitempty"`
	TemperatureLimitC     *float64 `json:"temperature_limit_celsius,omitempty"`
	PowerLimitWatts       *float64 `json:"power_limit_watts,omitempty"`
	PowerEnforcedLimitW   *float64 `json:"power_enforced_limit_watts,omitempty"`
	PCIeLinkGeneration    string   `json:"pcie_link_generation,omitempty"`
	PCIeLinkGenerationMax string   `json:"pcie_link_generation_max,omitempty"`
	PCIeLinkWidth         string   `json:"pcie_link_width,omitempty"`
	PCIeLinkWidthMax      string   `json:"pcie_link_width_max,omitempty"`
	ComputeMode           string   `json:"compute_mode,omitempty"`
	ComputeCapability     string   `json:"compute_capability,omitempty"`
	DisplayActive         string   `json:"display_active,omitempty"`
	DisplayAttached       string   `json:"display_attached,omitempty"`
	PersistenceMode       string   `json:"persistence_mode,omitempty"`
	DriverModel           string   `json:"driver_model,omitempty"`
	ECCModeCurrent        string   `json:"ecc_mode_current,omitempty"`
	MIGModeCurrent        string   `json:"mig_mode_current,omitempty"`
	ClockThrottleReasons  string   `json:"clock_throttle_reasons,omitempty"`
	CollectionError       string   `json:"collection_error,omitempty"`
}

type APIError struct {
	Error             string `json:"error"`
	RetryAfterSeconds int    `json:"retry_after_seconds,omitempty"`
}
