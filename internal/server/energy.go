package server

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	defaultEnergyCurrency         = "CNY"
	defaultThermalHotCelsius      = 85
	defaultIdleUtilizationPercent = 5
	defaultIdlePowerWatts         = 30
	maxEnergyCurrencyLength       = 12
	energySeriesMinuteRangeHours  = 24
	energyThermalCriticalDeltaC   = 5
	minDisplayIdleWasteKWh        = 0.005
)

type EnergySettings struct {
	EnergyPricePerKWh      float64 `json:"energy_price_per_kwh"`
	EnergyCurrency         string  `json:"energy_currency"`
	ThermalHotCelsius      float64 `json:"thermal_hot_celsius"`
	IdleUtilizationPercent float64 `json:"idle_utilization_percent"`
	IdlePowerWatts         float64 `json:"idle_power_watts"`
}

type energySummaryResponse struct {
	Hours       int                 `json:"hours"`
	Since       time.Time           `json:"since"`
	Until       time.Time           `json:"until"`
	Config      EnergySettings      `json:"config"`
	Summary     energySummary       `json:"summary"`
	Series      []energySeriesPoint `json:"series"`
	GPUs        []energyGPUStat     `json:"gpus"`
	Diagnostics []energyDiagnostic  `json:"diagnostics"`
}

type energySummary struct {
	CurrentPowerWatts     float64 `json:"current_power_watts"`
	AveragePowerWatts     float64 `json:"average_power_watts"`
	PeakPowerWatts        float64 `json:"peak_power_watts"`
	EnergyKWh             float64 `json:"energy_kwh"`
	EstimatedCost         float64 `json:"estimated_cost"`
	Currency              string  `json:"currency"`
	HotGPUCount           int     `json:"hot_gpu_count"`
	ThrottledGPUCount     int     `json:"throttled_gpu_count"`
	HighIdlePowerGPUCount int     `json:"high_idle_power_gpu_count"`
	IdleWasteKWh          float64 `json:"idle_waste_kwh"`
	CoveragePercent       float64 `json:"coverage_percent"`
	SampleCount           int     `json:"sample_count"`
	PowerSampleCount      int     `json:"power_sample_count"`
}

type energySeriesPoint struct {
	Timestamp              time.Time `json:"timestamp"`
	PowerWatts             float64   `json:"power_watts"`
	PeakTemperatureCelsius *float64  `json:"peak_temperature_celsius,omitempty"`
	HotGPUCount            int       `json:"hot_gpu_count"`
	GPUSampleCount         int       `json:"gpu_sample_count"`
}

type energyGPUStat struct {
	DeviceID               string    `json:"device_id"`
	DeviceAlias            string    `json:"device_alias,omitempty"`
	GPUID                  string    `json:"gpu_id"`
	GPUName                string    `json:"gpu_name"`
	SampleCount            int       `json:"sample_count"`
	PowerSampleCount       int       `json:"power_sample_count"`
	FirstSampleAt          time.Time `json:"first_sample_at,omitempty"`
	LastSampleAt           time.Time `json:"last_sample_at,omitempty"`
	CurrentPowerWatts      *float64  `json:"current_power_watts,omitempty"`
	AveragePowerWatts      *float64  `json:"average_power_watts,omitempty"`
	PeakPowerWatts         *float64  `json:"peak_power_watts,omitempty"`
	PeakTemperatureCelsius *float64  `json:"peak_temperature_celsius,omitempty"`
	EnergyKWh              float64   `json:"energy_kwh"`
	EstimatedCost          float64   `json:"estimated_cost"`
	HotSampleCount         int       `json:"hot_sample_count"`
	HotSeconds             int64     `json:"hot_seconds"`
	Throttled              bool      `json:"throttled"`
	ThrottleReason         string    `json:"throttle_reason,omitempty"`
	IdleWasteKWh           float64   `json:"idle_waste_kwh"`
	HighIdlePowerSeconds   int64     `json:"high_idle_power_seconds"`
	CoveragePercent        float64   `json:"coverage_percent"`
}

type energyDiagnostic struct {
	Kind        string  `json:"kind"`
	Severity    string  `json:"severity"`
	DeviceID    string  `json:"device_id,omitempty"`
	DeviceAlias string  `json:"device_alias,omitempty"`
	GPUID       string  `json:"gpu_id,omitempty"`
	GPUName     string  `json:"gpu_name,omitempty"`
	Value       float64 `json:"value,omitempty"`
	Unit        string  `json:"unit,omitempty"`
	Reason      string  `json:"reason,omitempty"`
}

type energyGPUIntegration struct {
	sampleCount       int
	powerSampleCount  int
	first             time.Time
	last              time.Time
	energyKWh         float64
	idleWasteKWh      float64
	observedSeconds   int64
	idleSeconds       int64
	hotSeconds        int64
	hotSampleCount    int
	peakPowerWatts    *float64
	peakTemperature   *float64
	averagePowerWatts *float64
}

type energySeriesBucket struct {
	timestamp   time.Time
	powerWatts  float64
	tempPeak    *float64
	hotCount    int
	sampleCount int
}

func (a *App) handleEnergySummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	hours := parseHours(r, 24)
	response, err := a.energySummary(hours, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *App) energySummary(hours int, now time.Time) (energySummaryResponse, error) {
	if hours <= 0 {
		hours = 24
	}
	settings := a.meta.ServiceConfig().EnergySettings()
	since := now.Add(-time.Duration(hours) * time.Hour)
	devices := a.meta.ListDevices()
	deviceByID := make(map[string]Device, len(devices))
	deviceIDs := make(map[string]bool, len(devices))
	for _, device := range devices {
		deviceByID[device.ID] = device
		deviceIDs[device.ID] = true
	}

	buckets := map[int64]*energySeriesBucket{}
	gpus := []energyGPUStat{}
	diagnostics := []energyDiagnostic{}
	summary := energySummary{Currency: settings.EnergyCurrency}
	rangeSeconds := int64(now.Sub(since).Seconds())
	if rangeSeconds <= 0 {
		rangeSeconds = int64(time.Hour.Seconds())
	}
	gapCap := energyGapCap(hours)

	for _, latest := range a.metrics.Latest() {
		if !deviceIDs[latest.DeviceID] {
			continue
		}
		device := deviceByID[latest.DeviceID]
		if latest.GPU.PowerDrawWatts != nil {
			summary.CurrentPowerWatts += *latest.GPU.PowerDrawWatts
		}
		if latest.GPU.TemperatureCelsius != nil && *latest.GPU.TemperatureCelsius >= settings.ThermalHotCelsius {
			summary.HotGPUCount++
		}
		if isActiveThrottleReason(latest.GPU.ClockThrottleReasons) {
			summary.ThrottledGPUCount++
		}

		points, err := energySeriesForGPU(a.metrics, latest.DeviceID, latest.GPU.GPUID, since, hours)
		if err != nil {
			return energySummaryResponse{}, err
		}
		points = mergeEnergyLatestPoint(points, latest, since)
		addEnergySeriesBuckets(buckets, points, settings, hours)
		integration := integrateEnergyPoints(points, settings, gapCap, rangeSeconds)
		idleWasteKWh := integration.idleWasteKWh
		idleSeconds := integration.idleSeconds
		if !hasDisplayableIdleWasteKWh(idleWasteKWh) {
			idleWasteKWh = 0
			idleSeconds = 0
		}
		stat := energyGPUStat{
			DeviceID:               latest.DeviceID,
			DeviceAlias:            deviceNameForEnergy(device, latest.DeviceID),
			GPUID:                  latest.GPU.GPUID,
			GPUName:                latest.GPU.Name,
			SampleCount:            integration.sampleCount,
			PowerSampleCount:       integration.powerSampleCount,
			FirstSampleAt:          integration.first,
			LastSampleAt:           integration.last,
			CurrentPowerWatts:      latest.GPU.PowerDrawWatts,
			AveragePowerWatts:      integration.averagePowerWatts,
			PeakPowerWatts:         integration.peakPowerWatts,
			PeakTemperatureCelsius: integration.peakTemperature,
			EnergyKWh:              integration.energyKWh,
			EstimatedCost:          integration.energyKWh * settings.EnergyPricePerKWh,
			HotSampleCount:         integration.hotSampleCount,
			HotSeconds:             integration.hotSeconds,
			Throttled:              isActiveThrottleReason(latest.GPU.ClockThrottleReasons),
			ThrottleReason:         strings.TrimSpace(latest.GPU.ClockThrottleReasons),
			IdleWasteKWh:           idleWasteKWh,
			HighIdlePowerSeconds:   idleSeconds,
			CoveragePercent:        integration.coveragePercent(rangeSeconds),
		}
		if stat.GPUName == "" {
			stat.GPUName = latest.GPU.GPUID
		}
		if hasDisplayableIdleWasteKWh(stat.IdleWasteKWh) {
			summary.HighIdlePowerGPUCount++
		}
		summary.EnergyKWh += stat.EnergyKWh
		summary.IdleWasteKWh += stat.IdleWasteKWh
		summary.SampleCount += stat.SampleCount
		summary.PowerSampleCount += stat.PowerSampleCount
		summary.CoveragePercent += stat.CoveragePercent
		gpus = append(gpus, stat)
		diagnostics = append(diagnostics, diagnosticsForEnergyGPU(stat, settings)...)
	}

	sort.Slice(gpus, func(i, j int) bool {
		if gpus[i].EnergyKWh == gpus[j].EnergyKWh {
			if gpus[i].DeviceID == gpus[j].DeviceID {
				return gpus[i].GPUID < gpus[j].GPUID
			}
			return gpus[i].DeviceID < gpus[j].DeviceID
		}
		return gpus[i].EnergyKWh > gpus[j].EnergyKWh
	})
	if len(gpus) > 0 {
		summary.CoveragePercent /= float64(len(gpus))
	}
	summary.EstimatedCost = summary.EnergyKWh * settings.EnergyPricePerKWh
	if hours > 0 {
		summary.AveragePowerWatts = summary.EnergyKWh * 1000 / float64(hours)
	}
	series := finishEnergySeries(buckets, hours)
	for _, point := range series {
		if point.PowerWatts > summary.PeakPowerWatts {
			summary.PeakPowerWatts = point.PowerWatts
		}
	}
	if summary.PeakPowerWatts == 0 {
		summary.PeakPowerWatts = summary.CurrentPowerWatts
	}
	sort.Slice(diagnostics, func(i, j int) bool {
		leftRank := energyDiagnosticSeverityRank(diagnostics[i].Severity)
		rightRank := energyDiagnosticSeverityRank(diagnostics[j].Severity)
		if leftRank == rightRank {
			return diagnostics[i].Kind < diagnostics[j].Kind
		}
		return leftRank > rightRank
	})

	return energySummaryResponse{
		Hours:       hours,
		Since:       since,
		Until:       now,
		Config:      settings,
		Summary:     summary,
		Series:      series,
		GPUs:        gpus,
		Diagnostics: diagnostics,
	}, nil
}

func validateEnergySettings(settings EnergySettings) error {
	if settings.EnergyPricePerKWh < 0 {
		return errors.New("energy price must be greater than or equal to 0")
	}
	currency := normalizeEnergyCurrency(settings.EnergyCurrency)
	if len(currency) > maxEnergyCurrencyLength {
		return errors.New("energy currency is too long")
	}
	for _, r := range currency {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return errors.New("energy currency may only contain letters, numbers, dash, or underscore")
	}
	if settings.ThermalHotCelsius <= 0 || settings.ThermalHotCelsius > 120 {
		return errors.New("thermal hot threshold must be between 1 and 120 Celsius")
	}
	if settings.IdleUtilizationPercent < 0 || settings.IdleUtilizationPercent > 100 {
		return errors.New("idle utilization threshold must be between 0 and 100 percent")
	}
	if settings.IdlePowerWatts < 0 || settings.IdlePowerWatts > 2000 {
		return errors.New("idle power threshold must be between 0 and 2000 watts")
	}
	return nil
}

func normalizeEnergyCurrency(currency string) string {
	currency = strings.TrimSpace(currency)
	if currency == "" {
		return defaultEnergyCurrency
	}
	return strings.ToUpper(currency)
}

func energySeriesForGPU(store *MetricsStore, deviceID, gpuID string, since time.Time, hours int) ([]SeriesPoint, error) {
	if hours > 1 {
		return store.SeriesRollup(deviceID, gpuID, since)
	}
	return store.Series(deviceID, gpuID, since)
}

func mergeEnergyLatestPoint(points []SeriesPoint, latest StoredGPU, since time.Time) []SeriesPoint {
	if latest.Timestamp.Before(since) {
		return points
	}
	point := SeriesPoint{
		Timestamp:             latest.Timestamp,
		UtilizationGPUPercent: latest.GPU.UtilizationGPUPercent,
		MemoryUsedBytes:       latest.GPU.MemoryUsedBytes,
		MemoryTotalBytes:      latest.GPU.MemoryTotalBytes,
		TemperatureCelsius:    latest.GPU.TemperatureCelsius,
		PowerDrawWatts:        latest.GPU.PowerDrawWatts,
	}
	out := append([]SeriesPoint(nil), points...)
	for index, current := range out {
		if current.Timestamp.Equal(point.Timestamp) {
			out[index] = point
			sortSeriesPoints(out)
			return out
		}
	}
	out = append(out, point)
	sortSeriesPoints(out)
	return out
}

func integrateEnergyPoints(points []SeriesPoint, settings EnergySettings, gapCap time.Duration, rangeSeconds int64) energyGPUIntegration {
	if len(points) == 0 {
		return energyGPUIntegration{}
	}
	clean := append([]SeriesPoint(nil), points...)
	sortSeriesPoints(clean)
	result := energyGPUIntegration{
		sampleCount: len(clean),
		first:       clean[0].Timestamp,
		last:        clean[len(clean)-1].Timestamp,
	}
	for _, point := range clean {
		if point.PowerDrawWatts != nil {
			result.powerSampleCount++
			if result.peakPowerWatts == nil || *point.PowerDrawWatts > *result.peakPowerWatts {
				value := *point.PowerDrawWatts
				result.peakPowerWatts = &value
			}
		}
		if point.TemperatureCelsius != nil {
			if result.peakTemperature == nil || *point.TemperatureCelsius > *result.peakTemperature {
				value := *point.TemperatureCelsius
				result.peakTemperature = &value
			}
			if *point.TemperatureCelsius >= settings.ThermalHotCelsius {
				result.hotSampleCount++
			}
		}
	}
	for index := 1; index < len(clean); index++ {
		previous := clean[index-1]
		current := clean[index]
		delta := current.Timestamp.Sub(previous.Timestamp)
		if delta <= 0 || delta > gapCap {
			continue
		}
		seconds := int64(delta.Seconds())
		if previous.PowerDrawWatts != nil && current.PowerDrawWatts != nil {
			avgPower := (*previous.PowerDrawWatts + *current.PowerDrawWatts) / 2
			segmentKWh := avgPower * delta.Hours() / 1000
			result.energyKWh += segmentKWh
			result.observedSeconds += seconds
			if isIdlePowerSegment(previous, current, settings) {
				result.idleWasteKWh += segmentKWh
				result.idleSeconds += seconds
			}
		}
		if isHotSegment(previous, current, settings) {
			result.hotSeconds += seconds
		}
	}
	if result.observedSeconds > 0 {
		value := result.energyKWh * 1000 / (float64(result.observedSeconds) / 3600)
		result.averagePowerWatts = &value
	}
	return result
}

func (g energyGPUIntegration) coveragePercent(rangeSeconds int64) float64 {
	if rangeSeconds <= 0 || g.observedSeconds <= 0 {
		return 0
	}
	value := float64(g.observedSeconds) / float64(rangeSeconds) * 100
	if value > 100 {
		return 100
	}
	return value
}

func energyGapCap(hours int) time.Duration {
	switch {
	case hours <= 1:
		return 5 * time.Minute
	case hours <= 24:
		return 15 * time.Minute
	default:
		return 2 * time.Hour
	}
}

func isIdlePowerSegment(previous, current SeriesPoint, settings EnergySettings) bool {
	if previous.PowerDrawWatts == nil || current.PowerDrawWatts == nil || previous.UtilizationGPUPercent == nil || current.UtilizationGPUPercent == nil {
		return false
	}
	avgPower := (*previous.PowerDrawWatts + *current.PowerDrawWatts) / 2
	avgUtil := (*previous.UtilizationGPUPercent + *current.UtilizationGPUPercent) / 2
	return avgPower >= settings.IdlePowerWatts && avgUtil <= settings.IdleUtilizationPercent
}

func isHotSegment(previous, current SeriesPoint, settings EnergySettings) bool {
	if previous.TemperatureCelsius == nil || current.TemperatureCelsius == nil {
		return false
	}
	avgTemp := (*previous.TemperatureCelsius + *current.TemperatureCelsius) / 2
	return avgTemp >= settings.ThermalHotCelsius
}

func addEnergySeriesBuckets(buckets map[int64]*energySeriesBucket, points []SeriesPoint, settings EnergySettings, hours int) {
	for _, point := range latestEnergyPointPerBucket(points, hours) {
		bucketAt := energySeriesBucketTime(point.Timestamp, hours)
		key := bucketAt.Unix()
		bucket := buckets[key]
		if bucket == nil {
			bucket = &energySeriesBucket{timestamp: bucketAt}
			buckets[key] = bucket
		}
		bucket.sampleCount++
		if point.PowerDrawWatts != nil {
			bucket.powerWatts += *point.PowerDrawWatts
		}
		if point.TemperatureCelsius != nil {
			if bucket.tempPeak == nil || *point.TemperatureCelsius > *bucket.tempPeak {
				value := *point.TemperatureCelsius
				bucket.tempPeak = &value
			}
			if *point.TemperatureCelsius >= settings.ThermalHotCelsius {
				bucket.hotCount++
			}
		}
	}
}

func latestEnergyPointPerBucket(points []SeriesPoint, hours int) []SeriesPoint {
	if len(points) <= 1 {
		return points
	}
	byBucket := map[int64]SeriesPoint{}
	for _, point := range points {
		key := energySeriesBucketTime(point.Timestamp, hours).Unix()
		current, ok := byBucket[key]
		if !ok || point.Timestamp.After(current.Timestamp) || point.Timestamp.Equal(current.Timestamp) {
			byBucket[key] = point
		}
	}
	out := make([]SeriesPoint, 0, len(byBucket))
	for _, point := range byBucket {
		out = append(out, point)
	}
	sortSeriesPoints(out)
	return out
}

func energySeriesBucketTime(timestamp time.Time, hours int) time.Time {
	if hours > energySeriesMinuteRangeHours {
		return timestamp.UTC().Truncate(time.Hour)
	}
	return timestamp.UTC().Truncate(time.Minute)
}

func finishEnergySeries(buckets map[int64]*energySeriesBucket, hours int) []energySeriesPoint {
	out := make([]energySeriesPoint, 0, len(buckets))
	for _, bucket := range buckets {
		out = append(out, energySeriesPoint{
			Timestamp:              bucket.timestamp,
			PowerWatts:             bucket.powerWatts,
			PeakTemperatureCelsius: bucket.tempPeak,
			HotGPUCount:            bucket.hotCount,
			GPUSampleCount:         bucket.sampleCount,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	if hours > energySeriesMinuteRangeHours {
		out = trimSparseEnergySeriesEdges(out)
	}
	return out
}

func trimSparseEnergySeriesEdges(points []energySeriesPoint) []energySeriesPoint {
	if len(points) < 3 {
		return points
	}
	maxSamples := 0
	for _, point := range points {
		if point.GPUSampleCount > maxSamples {
			maxSamples = point.GPUSampleCount
		}
	}
	if maxSamples < 2 {
		return points
	}
	start := 0
	end := len(points)
	for start < end && points[start].GPUSampleCount*2 < maxSamples {
		start++
	}
	for end > start && points[end-1].GPUSampleCount*2 < maxSamples {
		end--
	}
	if start == 0 && end == len(points) {
		return points
	}
	return append([]energySeriesPoint(nil), points[start:end]...)
}

func hasDisplayableIdleWasteKWh(value float64) bool {
	return value >= minDisplayIdleWasteKWh
}

func diagnosticsForEnergyGPU(stat energyGPUStat, settings EnergySettings) []energyDiagnostic {
	out := []energyDiagnostic{}
	if stat.PeakTemperatureCelsius != nil && *stat.PeakTemperatureCelsius >= settings.ThermalHotCelsius {
		severity := "warning"
		if *stat.PeakTemperatureCelsius >= settings.ThermalHotCelsius+energyThermalCriticalDeltaC {
			severity = "critical"
		}
		out = append(out, energyDiagnostic{
			Kind:        "thermal",
			Severity:    severity,
			DeviceID:    stat.DeviceID,
			DeviceAlias: stat.DeviceAlias,
			GPUID:       stat.GPUID,
			GPUName:     stat.GPUName,
			Value:       *stat.PeakTemperatureCelsius,
			Unit:        "celsius",
		})
	}
	if stat.Throttled {
		out = append(out, energyDiagnostic{
			Kind:        "throttle",
			Severity:    "warning",
			DeviceID:    stat.DeviceID,
			DeviceAlias: stat.DeviceAlias,
			GPUID:       stat.GPUID,
			GPUName:     stat.GPUName,
			Reason:      stat.ThrottleReason,
		})
	}
	if hasDisplayableIdleWasteKWh(stat.IdleWasteKWh) {
		out = append(out, energyDiagnostic{
			Kind:        "idle_waste",
			Severity:    "info",
			DeviceID:    stat.DeviceID,
			DeviceAlias: stat.DeviceAlias,
			GPUID:       stat.GPUID,
			GPUName:     stat.GPUName,
			Value:       stat.IdleWasteKWh,
			Unit:        "kwh",
		})
	}
	return out
}

func energyDiagnosticSeverityRank(severity string) int {
	switch severity {
	case "critical":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func isActiveThrottleReason(reason string) bool {
	reason = strings.TrimSpace(reason)
	if reason == "" || reason == "-" {
		return false
	}
	lower := strings.ToLower(reason)
	if lower == "none" || lower == "not active" || lower == "inactive" {
		return false
	}
	if strings.HasPrefix(lower, "0x") {
		for _, r := range lower[2:] {
			if r != '0' {
				return true
			}
		}
		return false
	}
	return true
}

func deviceNameForEnergy(device Device, fallback string) string {
	if device.Alias != "" {
		return device.Alias
	}
	if device.Hostname != "" {
		return device.Hostname
	}
	return fallback
}
