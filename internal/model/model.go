package model

import (
	"encoding/json"
	"fmt"
	"time"
)

type MetricStatus string

const (
	MetricStatusOK          MetricStatus = "ok"
	MetricStatusUnknown     MetricStatus = "unknown"
	MetricStatusUnsupported MetricStatus = "unsupported"
	MetricStatusError       MetricStatus = "error"
)

type MetricValue struct {
	Value  float64      `json:"value,omitempty"`
	Status MetricStatus `json:"status"`
}

func NewMetricValue(value float64) MetricValue {
	return MetricValue{
		Value:  value,
		Status: MetricStatusOK,
	}
}

func UnknownMetric() MetricValue {
	return MetricValue{Status: MetricStatusUnknown}
}

func UnsupportedMetric() MetricValue {
	return MetricValue{Status: MetricStatusUnsupported}
}

func ErrorMetric() MetricValue {
	return MetricValue{Status: MetricStatusError}
}

func (m MetricValue) IsKnown() bool {
	return m.Status == MetricStatusOK
}

func (m MetricValue) String(unit string, precision int) string {
	if !m.IsKnown() {
		return "n/a"
	}

	return fmt.Sprintf("%.*f%s", precision, m.Value, unit)
}

type CollectorError struct {
	Component string `json:"component"`
	Message   string `json:"message"`
	Fatal     bool   `json:"fatal"`
}

type HostSnapshot struct {
	HostID           string           `json:"host_id"`
	Hostname         string           `json:"hostname"`
	GPUBackend       string           `json:"gpu_backend,omitempty"`
	Timestamp        time.Time        `json:"timestamp"`
	CollectorLatency time.Duration    `json:"collector_latency_ns,omitempty"`
	SampleInterval   time.Duration    `json:"sample_interval_ns,omitempty"`
	UptimeSeconds    uint64           `json:"uptime_seconds"`
	CPUUsedPct       MetricValue      `json:"cpu_used_pct"`
	RAMUsedBytes     uint64           `json:"ram_used_bytes"`
	RAMTotalBytes    uint64           `json:"ram_total_bytes"`
	LoadAvg1         MetricValue      `json:"loadavg_1"`
	LoadAvg5         MetricValue      `json:"loadavg_5"`
	LoadAvg15        MetricValue      `json:"loadavg_15"`
	GPUs             []GPUSnapshot    `json:"gpus"`
	GPUProcesses     []GPUProcess     `json:"gpu_processes"`
	CollectorErrors  []CollectorError `json:"collector_errors,omitempty"`
}

func (h HostSnapshot) GPUCount() int {
	return len(h.GPUs)
}

func (h HostSnapshot) StaleAt(now time.Time) bool {
	if h.Timestamp.IsZero() || h.SampleInterval <= 0 {
		return false
	}

	return now.After(h.Timestamp.Add(h.SampleInterval * 3))
}

func (h HostSnapshot) MarshalJSON() ([]byte, error) {
	type alias HostSnapshot
	return json.Marshal(alias(h))
}

type GPUSnapshot struct {
	Index            int         `json:"index"`
	UUID             string      `json:"uuid"`
	Name             string      `json:"name"`
	GPUUtilPct       MetricValue `json:"gpu_util_pct"`
	MemUtilPct       MetricValue `json:"mem_util_pct"`
	MemoryUsedBytes  uint64      `json:"memory_used_bytes"`
	MemoryTotalBytes uint64      `json:"memory_total_bytes"`
	TemperatureC     MetricValue `json:"temperature_c"`
	FanPct           MetricValue `json:"fan_pct"`
	PowerW           MetricValue `json:"power_w"`
	PowerLimitW      MetricValue `json:"power_limit_w"`
	SMClockMHz       MetricValue `json:"sm_clock_mhz"`
	MemClockMHz      MetricValue `json:"mem_clock_mhz"`
	PState           string      `json:"pstate,omitempty"`
	IsMIGEnabled     bool        `json:"is_mig_enabled"`
	StatusFlags      []string    `json:"status_flags,omitempty"`
}

func (g GPUSnapshot) MemoryUsedPct() MetricValue {
	if g.MemoryTotalBytes == 0 {
		return UnknownMetric()
	}

	return NewMetricValue((float64(g.MemoryUsedBytes) / float64(g.MemoryTotalBytes)) * 100)
}

func (g GPUSnapshot) Key(hostID string) string {
	return fmt.Sprintf("%s/%s/%d", hostID, g.UUID, g.Index)
}

type GPUProcess struct {
	PID                int         `json:"pid"`
	User               string      `json:"user"`
	Command            string      `json:"command"`
	GPUUUID            string      `json:"gpu_uuid"`
	GPUIndex           int         `json:"gpu_index"`
	UsedGPUMemoryBytes uint64      `json:"used_gpu_memory_bytes"`
	SMUtilPct          MetricValue `json:"sm_util_pct"`
	MemUtilPct         MetricValue `json:"mem_util_pct"`
	ProcessRAMBytes    uint64      `json:"process_ram_bytes"`
	AgeSeconds         uint64      `json:"age_seconds"`
}

func (p GPUProcess) Key(hostID string) string {
	return fmt.Sprintf("%s/%d/%s", hostID, p.PID, p.GPUUUID)
}
