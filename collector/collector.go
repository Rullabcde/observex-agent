package collector

import (
	"os"
	"os/exec"
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

	// OS detection
	currentOS := runtime.GOOS
	metric.OS = currentOS

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

	// Disk usage
	diskPath := "/"
	if currentOS == "windows" {
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

	// Load Average (Linux only)
	if currentOS != "windows" {
		loadAvg, _ := load.Avg()
		if loadAvg != nil {
			metric.Load = models.LoadInfo{
				Load1:  loadAvg.Load1,
				Load5:  loadAvg.Load5,
				Load15: loadAvg.Load15,
			}
		}
	}

	// =======================================
	//  LOG COLLECTOR BAGIAN INI
	// =======================================
	metric.Logs = collectSystemLogs(currentOS)
	// =======================================

	return metric, nil
}

//
// ============================================================
//  LOG COLLECTOR (WINDOWS + LINUX) — SUDAH TERGABUNG
// ============================================================
//
func collectSystemLogs(osName string) models.LogsInfo {
	logs := models.LogsInfo{}

	switch osName {
	case "windows":
		logs.System = runPowerShell(`Get-EventLog -LogName System -Newest 50 | Out-String`)
		logs.Error = runPowerShell(`Get-EventLog -LogName Application -Newest 30 | Out-String`)
		logs.Security = runPowerShell(`Get-EventLog -LogName Security -Newest 30 | Out-String`)

	default:
		if _, err := os.Stat("/var/log/syslog"); err == nil {
			logs.System = readFile("/var/log/syslog")
		}
		if _, err := os.Stat("/var/log/messages"); err == nil {
			logs.Error = readFile("/var/log/messages")
		}

		logs.Security = runCmd("journalctl", "-n", "100")
	}

	return logs
}

//
// Helper functions
//
func runPowerShell(cmd string) string {
	out, _ := exec.Command("powershell", "-Command", cmd).CombinedOutput()
	return string(out)
}

func runCmd(name string, args ...string) string {
	out, _ := exec.Command(name, args...).CombinedOutput()
	return string(out)
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
