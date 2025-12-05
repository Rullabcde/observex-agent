package collector

import (
	"os"
	"runtime"
	"time"

	"observex-agent/models"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

func CollectMetrics() (*models.Metric, error) {
	metric := &models.Metric{
		Timestamp: time.Now(),
	}

	// Hostname
	hostname, _ := os.Hostname()
	metric.Hostname = hostname

	// System Info
	hostInfo, _ := host.Info()
	metric.System = models.SystemInfo{
		OS:     hostInfo.OS + " " + hostInfo.Platform + " " + hostInfo.PlatformVersion,
		Kernel: hostInfo.KernelVersion,
		Arch:   hostInfo.KernelArch,
	}

	// Uptime
	metric.Uptime = hostInfo.Uptime

	// CPU
	percent, _ := cpu.Percent(time.Second, false)
	cpuCount, _ := cpu.Counts(true)
	cpuInfo, _ := cpu.Info()
	model := ""
	if len(cpuInfo) > 0 {
		model = cpuInfo[0].ModelName
	}
	cpuPercent := 0.0
	if len(percent) > 0 {
		cpuPercent = percent[0]
	}
	metric.CPU = models.CPUInfo{
		Percent: cpuPercent,
		Cores:   cpuCount,
		Model:   model,
	}

	// Memory
	memInfo, _ := mem.VirtualMemory()
	metric.Memory = models.MemoryInfo{
		Total:     memInfo.Total,
		Available: memInfo.Available,
		Used:      memInfo.Used,
		Percent:   memInfo.UsedPercent,
	}

	// Swap
	swapInfo, _ := mem.SwapMemory()
	metric.Swap = models.SwapInfo{
		Total:   swapInfo.Total,
		Used:    swapInfo.Used,
		Percent: swapInfo.UsedPercent,
	}

	// Disk
	diskPath := "/"
	if runtime.GOOS == "windows" {
		diskPath = "C:\\"
	}
	diskUsage, _ := disk.Usage(diskPath)
	ioCounters, _ := disk.IOCounters()
	var readBytes, writeBytes uint64
	for _, counter := range ioCounters {
		readBytes += counter.ReadBytes
		writeBytes += counter.WriteBytes
	}
	metric.Disk = models.DiskInfo{
		Total:      diskUsage.Total,
		Free:       diskUsage.Free,
		Used:       diskUsage.Used,
		Percent:    diskUsage.UsedPercent,
		ReadBytes:  readBytes,
		WriteBytes: writeBytes,
	}

	// Network
	netIO, _ := net.IOCounters(false)
	if len(netIO) > 0 {
		metric.Network = models.NetworkInfo{
			BytesSent: netIO[0].BytesSent,
			BytesRecv: netIO[0].BytesRecv,
		}
	}

	// Load Average
	loadAvg, _ := load.Avg()
	if loadAvg != nil {
		metric.Load = models.LoadInfo{
			Load1:  loadAvg.Load1,
			Load5:  loadAvg.Load5,
			Load15: loadAvg.Load15,
		}
	}

	return metric, nil
}
