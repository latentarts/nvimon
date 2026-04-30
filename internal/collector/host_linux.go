package collector

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prods/nvimon/internal/model"
)

type procHostCollector struct {
	mu          sync.Mutex
	prevIdle    uint64
	prevTotal   uint64
	initialized bool
	readCPU     func() (uint64, uint64, error)
	sleep       func(context.Context, time.Duration) error
}

func newProcHostCollector() *procHostCollector {
	c := &procHostCollector{
		readCPU: readProcCPU,
		sleep:   sleepContext,
	}
	if total, idle, err := c.readCPU(); err == nil {
		c.prevTotal = total
		c.prevIdle = idle
		c.initialized = true
	}
	return c
}

func (c *procHostCollector) Collect(ctx context.Context) (hostSnapshot, error) {
	var snap hostSnapshot

	total, idle, err := c.readCPU()
	if err != nil {
		return snap, err
	}

	memTotal, memAvailable, err := readMemInfo()
	if err != nil {
		return snap, err
	}

	uptimeSeconds, err := readUptime()
	if err != nil {
		return snap, err
	}

	load1, load5, load15, err := readLoadAvg()
	if err != nil {
		return snap, err
	}

	snap.UptimeSeconds = uptimeSeconds
	snap.RAMTotalBytes = memTotal
	snap.RAMUsedBytes = memTotal - memAvailable
	snap.LoadAvg1 = model.NewMetricValue(load1)
	snap.LoadAvg5 = model.NewMetricValue(load5)
	snap.LoadAvg15 = model.NewMetricValue(load15)
	snap.CPUUsedPct = c.cpuUsage(total, idle)
	if !snap.CPUUsedPct.IsKnown() {
		if err := c.sleep(ctx, 120*time.Millisecond); err == nil {
			if total, idle, err := c.readCPU(); err == nil {
				snap.CPUUsedPct = c.cpuUsage(total, idle)
			}
		}
	}

	return snap, nil
}

func (c *procHostCollector) cpuUsage(total, idle uint64) model.MetricValue {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		c.prevTotal = total
		c.prevIdle = idle
		c.initialized = true
		return model.UnknownMetric()
	}

	deltaTotal := total - c.prevTotal
	deltaIdle := idle - c.prevIdle
	c.prevTotal = total
	c.prevIdle = idle

	if deltaTotal == 0 {
		return model.UnknownMetric()
	}

	usedPct := (float64(deltaTotal-deltaIdle) / float64(deltaTotal)) * 100
	return model.NewMetricValue(usedPct)
}

func readProcCPU() (uint64, uint64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, err
	}

	line := strings.SplitN(string(data), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, fmt.Errorf("unexpected /proc/stat format")
	}

	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		v, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, 0, err
		}
		values = append(values, v)
	}

	var total uint64
	for _, v := range values {
		total += v
	}

	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}

	return total, idle, nil
}

func readMemInfo() (uint64, uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}

	var totalKB uint64
	var availableKB uint64

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			totalKB, err = strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, 0, err
			}
		case "MemAvailable":
			availableKB, err = strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, 0, err
			}
		}
	}

	if totalKB == 0 {
		return 0, 0, fmt.Errorf("MemTotal missing from /proc/meminfo")
	}

	return totalKB * 1024, availableKB * 1024, nil
}

func readLoadAvg() (float64, float64, float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected /proc/loadavg format")
	}

	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	load5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	load15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	return load1, load5, load15, nil
}

func readUptime() (uint64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, fmt.Errorf("unexpected /proc/uptime format")
	}

	uptime, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}

	return uint64(uptime), nil
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
