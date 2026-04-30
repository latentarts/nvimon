package collector

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prods/nvimon/internal/model"
)

type commandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

type nvidiaSMICollector struct {
	runner commandRunner
}

func newNvidiaSMICollector() *nvidiaSMICollector {
	return &nvidiaSMICollector{runner: execRunner{}}
}

func (c *nvidiaSMICollector) Collect(ctx context.Context) (string, []model.GPUSnapshot, []model.GPUProcess, []model.CollectorError, error) {
	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		return "none", nil, nil, []model.CollectorError{{
			Component: "nvidia-smi",
			Message:   "nvidia-smi not found",
			Fatal:     false,
		}}, nil
	}

	gpuRows, err := c.runner.Run(ctx, "nvidia-smi",
		"--query-gpu=index,uuid,name,utilization.gpu,utilization.memory,memory.used,memory.total,temperature.gpu,fan.speed,power.draw,power.limit,clocks.sm,clocks.mem,pstate,mig.mode.current",
		"--format=csv,noheader,nounits",
	)
	if err != nil {
		return "nvidia-smi", nil, nil, nil, classifyNvidiaSMIError("gpu-query", err)
	}

	gpus, gpuIndexByUUID, err := parseGPUCSV(gpuRows)
	if err != nil {
		return "nvidia-smi", nil, nil, nil, err
	}
	if gpus == nil {
		gpus = []model.GPUSnapshot{}
	}

	processRows, err := c.runner.Run(ctx, "nvidia-smi",
		"--query-compute-apps=pid,gpu_uuid,used_gpu_memory",
		"--format=csv,noheader,nounits",
	)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "nvidia-smi", gpus, nil, []model.CollectorError{{
				Component: "nvidia-smi-processes",
				Message:   classifyNvidiaSMIProcessError(exitErr),
				Fatal:     false,
			}}, nil
		}
		return "nvidia-smi", gpus, nil, nil, classifyNvidiaSMIError("process-query", err)
	}

	processes, errs := parseProcessCSV(processRows, gpuIndexByUUID)
	if processes == nil {
		processes = []model.GPUProcess{}
	}
	return "nvidia-smi", gpus, processes, errs, nil
}

func parseGPUCSV(data []byte) ([]model.GPUSnapshot, map[string]int, error) {
	lines := nonEmptyLines(string(data))
	reader := csv.NewReader(strings.NewReader(strings.Join(lines, "\n")))
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(records) == 0 {
		return []model.GPUSnapshot{}, map[string]int{}, nil
	}

	gpus := make([]model.GPUSnapshot, 0, len(records))
	indexByUUID := make(map[string]int, len(records))

	for _, rec := range records {
		if len(rec) < 15 {
			return nil, nil, fmt.Errorf("unexpected gpu csv field count: %d", len(rec))
		}

		index, err := strconv.Atoi(strings.TrimSpace(rec[0]))
		if err != nil {
			return nil, nil, err
		}

		uuid := strings.TrimSpace(rec[1])
		gpu := model.GPUSnapshot{
			Index:            index,
			UUID:             uuid,
			Name:             strings.TrimSpace(rec[2]),
			GPUUtilPct:       parseMetric(rec[3]),
			MemUtilPct:       parseMetric(rec[4]),
			MemoryUsedBytes:  parseMiBToBytes(rec[5]),
			MemoryTotalBytes: parseMiBToBytes(rec[6]),
			TemperatureC:     parseMetric(rec[7]),
			FanPct:           parseMetric(rec[8]),
			PowerW:           parseMetric(rec[9]),
			PowerLimitW:      parseMetric(rec[10]),
			SMClockMHz:       parseMetric(rec[11]),
			MemClockMHz:      parseMetric(rec[12]),
			PState:           strings.TrimSpace(rec[13]),
			IsMIGEnabled:     strings.EqualFold(strings.TrimSpace(rec[14]), "Enabled"),
		}

		gpus = append(gpus, gpu)
		indexByUUID[uuid] = index
	}

	return gpus, indexByUUID, nil
}

func parseProcessCSV(data []byte, gpuIndexByUUID map[string]int) ([]model.GPUProcess, []model.CollectorError) {
	lines := nonEmptyLines(string(data))
	if len(lines) == 0 || (len(lines) == 1 && strings.Contains(lines[0], "No running compute processes found")) {
		return []model.GPUProcess{}, nil
	}

	reader := csv.NewReader(strings.NewReader(strings.Join(lines, "\n")))
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, []model.CollectorError{{
			Component: "nvidia-smi-processes",
			Message:   err.Error(),
			Fatal:     false,
		}}
	}

	processes := make([]model.GPUProcess, 0, len(records))
	var collectorErrors []model.CollectorError

	for _, rec := range records {
		if len(rec) < 3 {
			continue
		}

		pid, err := strconv.Atoi(strings.TrimSpace(rec[0]))
		if err != nil {
			collectorErrors = append(collectorErrors, model.CollectorError{
				Component: "nvidia-smi-processes",
				Message:   fmt.Sprintf("invalid pid %q", rec[0]),
				Fatal:     false,
			})
			continue
		}

		uuid := strings.TrimSpace(rec[1])
		process, err := enrichProcess(pid)
		if err != nil {
			collectorErrors = append(collectorErrors, model.CollectorError{
				Component: "procfs",
				Message:   fmt.Sprintf("pid %d: %v", pid, err),
				Fatal:     false,
			})
			process = model.GPUProcess{PID: pid}
		}

		process.GPUUUID = uuid
		process.GPUIndex = gpuIndexByUUID[uuid]
		process.UsedGPUMemoryBytes = parseMiBToBytes(rec[2])
		process.SMUtilPct = model.UnknownMetric()
		process.MemUtilPct = model.UnknownMetric()
		processes = append(processes, process)
	}

	return processes, collectorErrors
}

func enrichProcess(pid int) (model.GPUProcess, error) {
	proc := model.GPUProcess{PID: pid}

	cmdline, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err == nil {
		proc.Command = strings.ReplaceAll(strings.TrimRight(string(cmdline), "\x00"), "\x00", " ")
	}

	status, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return proc, err
	}

	var uid string
	for _, line := range strings.Split(string(status), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch strings.TrimSuffix(fields[0], ":") {
		case "Uid":
			uid = fields[1]
		case "VmRSS":
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				proc.ProcessRAMBytes = v * 1024
			}
		case "Name":
			if proc.Command == "" && len(fields) >= 2 {
				proc.Command = strings.Join(fields[1:], " ")
			}
		}
	}

	if uid != "" {
		proc.User = resolveUsername(uid)
	}

	if stat, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); err == nil {
		if sys, ok := stat.Sys().(*syscall.Stat_t); ok {
			_ = sys
		}
	}

	if info, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); err == nil {
		age := time.Since(info.ModTime())
		if age > 0 {
			proc.AgeSeconds = uint64(age.Seconds())
		}
	}

	if proc.Command == "" {
		proc.Command = "unknown"
	}

	return proc, nil
}

func resolveUsername(uid string) string {
	passwd, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return uid
	}

	for _, line := range strings.Split(string(passwd), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) > 2 && fields[2] == uid {
			return fields[0]
		}
	}

	return uid
}

func parseMetric(value string) model.MetricValue {
	s := strings.TrimSpace(value)
	switch {
	case s == "", strings.EqualFold(s, "N/A"), strings.EqualFold(s, "[N/A]"):
		return model.UnknownMetric()
	case strings.Contains(strings.ToLower(s), "not supported"):
		return model.UnsupportedMetric()
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return model.UnknownMetric()
	}

	return model.NewMetricValue(v)
}

func parseMiBToBytes(value string) uint64 {
	s := strings.TrimSpace(value)
	switch {
	case s == "", strings.EqualFold(s, "N/A"), strings.Contains(strings.ToLower(s), "not supported"):
		return 0
	}

	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}

	return v * 1024 * 1024
}

func nonEmptyLines(raw string) []string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func classifyNvidiaSMIError(op string, err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		if stderr != "" {
			return fmt.Errorf("%s failed: %s", op, stderr)
		}
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "exit status 9"):
		return fmt.Errorf("%s failed: nvidia-smi could not communicate with the NVIDIA driver or no NVIDIA devices are available", op)
	case strings.Contains(msg, "exit status 14"):
		return fmt.Errorf("%s failed: nvidia-smi reported insufficient permission", op)
	default:
		return fmt.Errorf("%s failed: %s", op, msg)
	}
}

func classifyNvidiaSMIProcessError(exitErr *exec.ExitError) string {
	stderr := strings.TrimSpace(string(exitErr.Stderr))
	if stderr != "" {
		return stderr
	}

	msg := exitErr.Error()
	switch {
	case strings.Contains(msg, "exit status 9"):
		return "nvidia-smi process query could not communicate with the NVIDIA driver or no NVIDIA devices are available"
	case strings.Contains(msg, "exit status 14"):
		return "nvidia-smi process query reported insufficient permission"
	default:
		return msg
	}
}
