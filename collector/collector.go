package collector

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"observex-agent/models"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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

// collectSystemLogs collects system, error, and security logs
func collectSystemLogs(osName string) models.LogsInfo {
	logs := models.LogsInfo{}
	switch osName {
	case "windows":
		// Windows logic is already good
		logs.System = runPowerShell(`Get-EventLog -LogName System -Newest 50 | Out-String`)
		// logs.Error removed
		logs.Security = runPowerShell(`Get-EventLog -LogName Security -Newest 30 | Out-String`)
	default: // Linux/Darwin
		// Try Journalctl first
		
		// 1. System Logs
		sysLog, err := runCmdWithErr("journalctl", "-k", "-b", "-n", "50", "--no-pager", "-o", "cat")
		if err == nil && len(sysLog) > 10 {
			logs.System = sysLog
		} else {
			// Fallback: Text files
			paths := []string{
				"/host/var/log/syslog", "/var/log/syslog",
				"/host/var/log/messages", "/var/log/messages",
			}
			for _, path := range paths {
				if _, err := os.Stat(path); err == nil {
					logs.System = runCmd("tail", "-n", "50", path)
					break
				}
			}
		}
		// 3. Security Logs
		secLog, err := runCmdWithErr("journalctl", "_COMM=sshd", "-n", "50", "--no-pager", "-o", "cat") 
		if err == nil && len(secLog) > 10 {
			logs.Security = secLog
		} else {
			// Fallback: auth logs
			paths := []string{
				"/host/var/log/auth.log", "/var/log/auth.log",
				"/host/var/log/secure", "/var/log/secure", // RHEL/CentOS
			}
			for _, path := range paths {
				if _, err := os.Stat(path); err == nil {
					logs.Security = runCmd("tail", "-n", "50", path)
					break
				}
			}
		}
	}
	return logs
}

// collectDockerContainers collects list of running containers
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

		logs := collectContainerLogs(cli, c.ID)

		containers = append(containers, models.ContainerInfo{
			ID:      c.ID[:12], // Short ID
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Created: c.Created,
			Logs:    logs,
		})
	}

	return containers
}

func collectContainerLogs(cli *client.Client, containerID string) string {
    ctx := context.Background()
    
    options := container.LogsOptions{
        ShowStdout: true,
        ShowStderr: true,
        Tail:       "100",
        Timestamps: true,
    }
    
    reader, err := cli.ContainerLogs(ctx, containerID, options)
    if err != nil {
        return ""
    }
    defer reader.Close()
    
    // Read logs (handle multiplexed stream)
    var buf bytes.Buffer
    _, _ = stdcopy.StdCopy(&buf, &buf, reader)
    
    return buf.String()
}

func runPowerShell(cmd string) string {
	out, _ := exec.Command("powershell", "-Command", cmd).CombinedOutput()
	return string(out)
}

// Helper that returns error so we know when to fallback
func runCmdWithErr(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

func runCmd(name string, args ...string) string {
	out, _ := exec.Command(name, args...).CombinedOutput()
	return string(out)
}

