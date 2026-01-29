package collector

import (
	"time"

	"observex-agent/models"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

// Gathers OS and Uptime info
func collectSystemInfo(metric *models.Metric) {
	if hostInfo, err := host.Info(); err == nil {
		metric.System = models.SystemInfo{
			OS:     hostInfo.OS + " " + hostInfo.Platform + " " + hostInfo.PlatformVersion,
			Kernel: hostInfo.KernelVersion,
			Arch:   hostInfo.KernelArch,
		}
		metric.Uptime = hostInfo.Uptime
	}
}

// Gathers CPU stats
func collectCPUInfo(metric *models.Metric) {
	if percent, err := cpu.Percent(time.Second, false); err == nil && len(percent) > 0 {
		metric.CPU.Percent = percent[0]
	}
	if count, err := cpu.Counts(true); err == nil {
		metric.CPU.Cores = count
	}
	if info, err := cpu.Info(); err == nil && len(info) > 0 {
		metric.CPU.Model = info[0].ModelName
	}
}

// Gathers Memory and Swap stats
func collectMemoryInfo(metric *models.Metric) {
	if memInfo, err := mem.VirtualMemory(); err == nil {
		metric.Memory = models.MemoryInfo{
			Total:     memInfo.Total,
			Available: memInfo.Available,
			Used:      memInfo.Used,
			Percent:   memInfo.UsedPercent,
		}
	}

	if swapInfo, err := mem.SwapMemory(); err == nil {
		metric.Swap = models.SwapInfo{
			Total:   swapInfo.Total,
			Used:    swapInfo.Used,
			Percent: swapInfo.UsedPercent,
		}
	}
}

// Gathers Disk Usage and I/O stats
func collectDiskInfo(metric *models.Metric, currentOS string) {
	diskPath := "/"
	if currentOS == "windows" {
		diskPath = "C:\\"
	}
	if diskUsage, err := disk.Usage(diskPath); err == nil {
		metric.Disk.Total = diskUsage.Total
		metric.Disk.Free = diskUsage.Free
		metric.Disk.Used = diskUsage.Used
		metric.Disk.Percent = diskUsage.UsedPercent
	}

	if ioCounters, err := disk.IOCounters(); err == nil {
		for _, counter := range ioCounters {
			metric.Disk.ReadBytes += counter.ReadBytes
			metric.Disk.WriteBytes += counter.WriteBytes
		}
	}
}

// Gathers Load Average (Unix only)
func collectLoadInfo(metric *models.Metric, currentOS string) {
	if currentOS != "windows" {
		if loadAvg, err := load.Avg(); err == nil && loadAvg != nil {
			metric.Load = models.LoadInfo{
				Load1:  loadAvg.Load1,
				Load5:  loadAvg.Load5,
				Load15: loadAvg.Load15,
			}
		}
	}
}
