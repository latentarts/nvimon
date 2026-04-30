//go:build cgo

package collector

import (
	"context"
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml"

	"github.com/prods/nvimon/internal/model"
)

type nvmlLibrary interface {
	Init() nvml.Return
	Shutdown() nvml.Return
	DeviceGetCount() (int, nvml.Return)
	DeviceGetHandleByIndex(int) (nvml.Device, nvml.Return)
}

type nvmlCollector struct {
	lib nvmlLibrary
}

func newNVMLCollector() *nvmlCollector {
	return &nvmlCollector{lib: nvml.New()}
}

func (c *nvmlCollector) Collect(ctx context.Context) (string, []model.GPUSnapshot, []model.GPUProcess, []model.CollectorError, error) {
	_ = ctx

	if ret := c.lib.Init(); ret != nvml.SUCCESS {
		return "nvml", nil, nil, nil, fmt.Errorf("init nvml: %s", ret.Error())
	}
	defer c.lib.Shutdown()

	count, ret := c.lib.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return "nvml", nil, nil, nil, fmt.Errorf("count devices: %s", ret.Error())
	}

	gpus := make([]model.GPUSnapshot, 0, count)
	processes := make([]model.GPUProcess, 0)
	collectorErrors := make([]model.CollectorError, 0)

	for i := 0; i < count; i++ {
		device, ret := c.lib.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			collectorErrors = append(collectorErrors, model.CollectorError{
				Component: "nvml",
				Message:   fmt.Sprintf("gpu %d handle: %s", i, ret.Error()),
				Fatal:     false,
			})
			continue
		}

		gpu, gpuErrs := collectNVMLDevice(device, i)
		gpus = append(gpus, gpu)
		collectorErrors = append(collectorErrors, gpuErrs...)

		deviceProcesses, processErrs := collectNVMLProcesses(device, gpu)
		processes = append(processes, deviceProcesses...)
		collectorErrors = append(collectorErrors, processErrs...)
	}

	return "nvml", gpus, processes, collectorErrors, nil
}

func collectNVMLDevice(device nvml.Device, index int) (model.GPUSnapshot, []model.CollectorError) {
	errors := make([]model.CollectorError, 0)
	gpu := model.GPUSnapshot{Index: index}

	if name, ret := device.GetName(); ret == nvml.SUCCESS {
		gpu.Name = name
	} else {
		errors = append(errors, nvmlMetricError(index, "name", ret))
		gpu.Name = fmt.Sprintf("GPU %d", index)
	}

	if uuid, ret := device.GetUUID(); ret == nvml.SUCCESS {
		gpu.UUID = uuid
	} else {
		errors = append(errors, nvmlMetricError(index, "uuid", ret))
		gpu.UUID = fmt.Sprintf("GPU-%d", index)
	}

	if util, ret := device.GetUtilizationRates(); ret == nvml.SUCCESS {
		gpu.GPUUtilPct = model.NewMetricValue(float64(util.Gpu))
		gpu.MemUtilPct = model.NewMetricValue(float64(util.Memory))
	} else {
		gpu.GPUUtilPct = metricFromNVMLReturn(ret)
		gpu.MemUtilPct = metricFromNVMLReturn(ret)
	}

	if memory, ret := device.GetMemoryInfo(); ret == nvml.SUCCESS {
		gpu.MemoryUsedBytes = memory.Used
		gpu.MemoryTotalBytes = memory.Total
	}

	if temp, ret := device.GetTemperature(nvml.TEMPERATURE_GPU); ret == nvml.SUCCESS {
		gpu.TemperatureC = model.NewMetricValue(float64(temp))
	} else {
		gpu.TemperatureC = metricFromNVMLReturn(ret)
	}

	if fan, ret := device.GetFanSpeed(); ret == nvml.SUCCESS {
		gpu.FanPct = model.NewMetricValue(float64(fan))
	} else {
		gpu.FanPct = metricFromNVMLReturn(ret)
	}

	if power, ret := device.GetPowerUsage(); ret == nvml.SUCCESS {
		gpu.PowerW = model.NewMetricValue(float64(power) / 1000.0)
	} else {
		gpu.PowerW = metricFromNVMLReturn(ret)
	}

	if limit, ret := device.GetEnforcedPowerLimit(); ret == nvml.SUCCESS {
		gpu.PowerLimitW = model.NewMetricValue(float64(limit) / 1000.0)
	} else {
		gpu.PowerLimitW = metricFromNVMLReturn(ret)
	}

	if smClock, ret := device.GetClockInfo(nvml.CLOCK_SM); ret == nvml.SUCCESS {
		gpu.SMClockMHz = model.NewMetricValue(float64(smClock))
	} else {
		gpu.SMClockMHz = metricFromNVMLReturn(ret)
	}

	if memClock, ret := device.GetClockInfo(nvml.CLOCK_MEM); ret == nvml.SUCCESS {
		gpu.MemClockMHz = model.NewMetricValue(float64(memClock))
	} else {
		gpu.MemClockMHz = metricFromNVMLReturn(ret)
	}

	if pstate, ret := device.GetPerformanceState(); ret == nvml.SUCCESS {
		gpu.PState = fmt.Sprintf("P%d", int(pstate))
	}

	return gpu, errors
}

func collectNVMLProcesses(device nvml.Device, gpu model.GPUSnapshot) ([]model.GPUProcess, []model.CollectorError) {
	procInfos, ret := device.GetComputeRunningProcesses()
	if ret != nvml.SUCCESS {
		return nil, []model.CollectorError{{
			Component: "nvml-processes",
			Message:   fmt.Sprintf("gpu %d processes: %s", gpu.Index, ret.Error()),
			Fatal:     false,
		}}
	}

	processes := make([]model.GPUProcess, 0, len(procInfos))
	errors := make([]model.CollectorError, 0)
	for _, info := range procInfos {
		process, err := enrichProcess(int(info.Pid))
		if err != nil {
			errors = append(errors, model.CollectorError{
				Component: "procfs",
				Message:   fmt.Sprintf("pid %d: %v", info.Pid, err),
				Fatal:     false,
			})
			process = model.GPUProcess{PID: int(info.Pid)}
		}

		process.GPUUUID = gpu.UUID
		process.GPUIndex = gpu.Index
		process.UsedGPUMemoryBytes = info.UsedGpuMemory
		process.SMUtilPct = model.UnknownMetric()
		process.MemUtilPct = model.UnknownMetric()
		processes = append(processes, process)
	}

	return processes, errors
}

func metricFromNVMLReturn(ret nvml.Return) model.MetricValue {
	switch ret {
	case nvml.SUCCESS:
		return model.NewMetricValue(0)
	case nvml.ERROR_NOT_SUPPORTED:
		return model.UnsupportedMetric()
	default:
		return model.UnknownMetric()
	}
}

func nvmlMetricError(index int, field string, ret nvml.Return) model.CollectorError {
	return model.CollectorError{
		Component: "nvml",
		Message:   fmt.Sprintf("gpu %d %s: %s", index, field, ret.Error()),
		Fatal:     false,
	}
}
