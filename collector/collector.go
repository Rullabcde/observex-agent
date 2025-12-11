package collector

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"observex-agent/models"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

func CollectMetrics() (*models.Metric, error) {
	metric := &models.Metric{Timestamp: time.Now()}
	currentOS := runtime.GOOS
	metric.OS = currentOS

	hostname, _ := os.Hostname()
	metric.Hostname = hostname

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
	metric.CPU = models.CPUInfo{Percent: cpuPercent, Cores: cpuCount, Model: model}

	// Memory
	memInfo, _ := mem.VirtualMemory()
	metric.Memory = models.MemoryInfo{
		Total: memInfo.Total, Available: memInfo.Available,
		Used: memInfo.Used, Percent: memInfo.UsedPercent,
	}

	// Swap
	swapInfo, _ := mem.SwapMemory()
	metric.Swap = models.SwapInfo{Total: swapInfo.Total, Used: swapInfo.Used, Percent: swapInfo.UsedPercent}

	// Disk
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
		Total: diskUsage.Total, Free: diskUsage.Free, Used: diskUsage.Used,
		Percent: diskUsage.UsedPercent, ReadBytes: readBytes, WriteBytes: writeBytes,
	}

	// Network
	netIO, _ := net.IOCounters(false)
	if len(netIO) > 0 {
		metric.Network = models.NetworkInfo{BytesSent: netIO[0].BytesSent, BytesRecv: netIO[0].BytesRecv}
	}

	// Load Average
	if currentOS != "windows" {
		loadAvg, _ := load.Avg()
		if loadAvg != nil {
			metric.Load = models.LoadInfo{Load1: loadAvg.Load1, Load5: loadAvg.Load5, Load15: loadAvg.Load15}
		}
	}

	// LOGS - Always read host logs (native or mounted)
	metric.Logs = collectSystemLogs(currentOS)

	// CONTAINERS - Collect Docker container list if docker.sock available
	metric.Containers = collectDockerContainers()

	return metric, nil
}

// collectSystemLogs kumpulin log sistem
func collectSystemLogs(osName string) models.LogsInfo {
	logs := models.LogsInfo{}

	switch osName {
	case "windows":
		logs.System = runPowerShell(`Get-EventLog -LogName System -Newest 50 | Out-String`)
		logs.Error = runPowerShell(`Get-EventLog -LogName Application -Newest 30 | Out-String`)
		logs.Security = runPowerShell(`Get-EventLog -LogName Security -Newest 30 | Out-String`)

	default:
		// Detect if running in Docker (mounted host logs at /host/var/log)
		isDocker := false
		if _, err := os.Stat("/host/var/log"); err == nil {
			isDocker = true
		}

		// System logs - syslog
		syslogPath := "/var/log/syslog"
		if isDocker {
			syslogPath = "/host/var/log/syslog"
		}
		if _, err := os.Stat(syslogPath); err == nil {
			logs.System = runCmd("tail", "-n", "100", syslogPath)
		} else {
			// Fallback: try messages (RHEL/CentOS)
			messagesPath := "/var/log/messages"
			if isDocker {
				messagesPath = "/host/var/log/messages"
			}
			if _, err := os.Stat(messagesPath); err == nil {
				logs.System = runCmd("tail", "-n", "100", messagesPath)
			}
		}

		// Error logs - journalctl with priority error
		if isDocker {
			// In Docker, journalctl needs mounted journal
			logs.Error = runCmd("journalctl", "--directory=/host/run/log/journal", "-p", "err", "-n", "50", "--no-pager")
		} else {
			logs.Error = runCmd("journalctl", "-p", "err", "-n", "50", "--no-pager")
		}

		// Security logs - recent journal entries
		if isDocker {
			logs.Security = runCmd("journalctl", "--directory=/host/run/log/journal", "-n", "100", "--no-pager")
		} else {
			logs.Security = runCmd("journalctl", "-n", "100", "--no-pager")
		}
	}

	return logs
}

// collectDockerContainers ambil list semua container
func collectDockerContainers() []models.ContainerInfo {
	containers := []models.ContainerInfo{}

	// Check if docker.sock is available
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		return containers // No Docker socket, return empty
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return containers
	}
	defer cli.Close()

	ctx := context.Background()
	containerList, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return containers
	}

	for _, c := range containerList {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		containers = append(containers, models.ContainerInfo{
			ID:      c.ID[:12], // Short ID
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Created: c.Created,
		})
	}

	return containers
}

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
	// Limit to last 50KB to avoid huge payloads
	if len(data) > 50000 {
		data = data[len(data)-50000:]
	}
	return string(data)
}