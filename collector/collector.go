package collector

import (
	"context"
	"io"
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

// CollectMetrics kumpulin semua metrik sistem
func CollectMetrics() (*models.Metric, error) {
	metric := &models.Metric{
		Timestamp: time.Now(),
	}

	// Deteksi sistem operasi (linux, windows, darwin)
	currentOS := runtime.GOOS
	metric.OS = currentOS

	// Hostname & System Info
	hostname, _ := os.Hostname()
	metric.Hostname = hostname

	hostInfo, _ := host.Info()
	metric.System = models.SystemInfo{
		OS:     hostInfo.OS + " " + hostInfo.Platform + " " + hostInfo.PlatformVersion,
		Kernel: hostInfo.KernelVersion,
		Arch:   hostInfo.KernelArch,
	}
	metric.Uptime = hostInfo.Uptime

	// CPU usage
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

	// Memory/RAM
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

	// Disk usage (Windows pake C:\, Linux pake /)
	diskPath := "/"
	if currentOS == "windows" {
		diskPath = "C:\\"
	}

	diskUsage, _ := disk.Usage(diskPath)

	// Disk I/O (total semua disk)
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

	// Network I/O (aggregated semua interface)
	netIO, _ := net.IOCounters(false)
	if len(netIO) > 0 {
		metric.Network = models.NetworkInfo{
			BytesSent: netIO[0].BytesSent,
			BytesRecv: netIO[0].BytesRecv,
		}
	}

	// Load Average (Linux only, Windows gak support)
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

	// Kumpulin logs
	metric.Logs = collectSystemLogs(currentOS)

	// Kumpulin container info (kalo ada Docker)
	metric.Containers = collectContainerInfo()

	return metric, nil
}

// collectSystemLogs kumpulin log berdasarkan OS/environment
func collectSystemLogs(osName string) models.LogsInfo {
	logs := models.LogsInfo{}

	// Cek dulu ada docker gak
	if isDockerEnvironment() {
		return collectDockerLogs()
	}

	switch osName {
	case "windows":
		// Windows pake PowerShell buat akses Event Log
		logs.System = runPowerShell(`Get-EventLog -LogName System -Newest 50 | Out-String`)
		logs.Error = runPowerShell(`Get-EventLog -LogName Application -Newest 30 | Out-String`)
		logs.Security = runPowerShell(`Get-EventLog -LogName Security -Newest 30 | Out-String`)

	default:
		// Linux/Unix pake syslog/journalctl
		if _, err := os.Stat("/var/log/syslog"); err == nil {
			logs.System = runCmd("tail", "-n", "100", "/var/log/syslog")
		} else if _, err := os.Stat("/var/log/messages"); err == nil {
			logs.System = runCmd("tail", "-n", "100", "/var/log/messages")
		}

		logs.Error = runCmd("journalctl", "-p", "err", "-n", "50", "--no-pager")
		logs.Security = runCmd("journalctl", "-n", "100", "--no-pager")
	}

	return logs
}

// isDockerEnvironment cek ada docker.sock gak
func isDockerEnvironment() bool {
	_, err := os.Stat("/var/run/docker.sock")
	return err == nil
}

// collectDockerLogs ambil log dari semua running containers
func collectDockerLogs() models.LogsInfo {
	logs := models.LogsInfo{}

	// Connect ke Docker daemon
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logs.Error = "Docker connect failed: " + err.Error()
		return logs
	}
	defer cli.Close()

	ctx := context.Background()

	// List semua container yang running
	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		logs.Error = "Container list failed: " + err.Error()
		return logs
	}

	var systemLogs, errorLogs strings.Builder

	for _, c := range containers {
		// Ambil nama container
		name := "unknown"
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		// Ambil 30 line terakhir dari container
		reader, err := cli.ContainerLogs(ctx, c.ID, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Tail:       "30",
			Timestamps: true,
		})
		if err != nil {
			continue
		}

		logBytes, _ := io.ReadAll(reader)
		reader.Close()

		logContent := string(logBytes)
		systemLogs.WriteString("\n=== " + name + " ===\n" + logContent)

		// Filter yang ada error/fatal
		for _, line := range strings.Split(logContent, "\n") {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") {
				errorLogs.WriteString(name + ": " + line + "\n")
			}
		}
	}

	logs.System = systemLogs.String()
	logs.Error = errorLogs.String()
	logs.Security = runCmd("docker", "events", "--since", "5m", "--until", "now")

	return logs
}

// collectContainerInfo ambil info semua container (running + stopped)
func collectContainerInfo() []models.ContainerInfo {
	// Cek dulu ada docker gak
	if !isDockerEnvironment() {
		return nil
	}

	// Connect ke Docker daemon
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil
	}
	defer cli.Close()

	ctx := context.Background()

	// List semua container (All: true = termasuk yang stopped)
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil
	}

	var result []models.ContainerInfo
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		result = append(result, models.ContainerInfo{
			ID:      c.ID[:12], // Short ID
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Created: c.Created,
		})
	}

	return result
}

// runPowerShell jalanin command PowerShell
func runPowerShell(cmd string) string {
	out, _ := exec.Command("powershell", "-Command", cmd).CombinedOutput()
	return string(out)
}

// runCmd jalanin command shell biasa
func runCmd(name string, args ...string) string {
	out, _ := exec.Command(name, args...).CombinedOutput()
	return string(out)
}