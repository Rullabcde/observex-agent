package collector

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
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

// CollectMetrics gathers all system metrics
func CollectMetrics() (*models.Metric, error) {
	timestamp := time.Now()
	currentOS := runtime.GOOS

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	metric := &models.Metric{
		Timestamp: timestamp,
		OS:        currentOS,
		Hostname:  hostname,
		PublicIP:  getPublicIP(),
	}

	// System Info
	if hostInfo, err := host.Info(); err == nil {
		metric.System = models.SystemInfo{
			OS:     hostInfo.OS + " " + hostInfo.Platform + " " + hostInfo.PlatformVersion,
			Kernel: hostInfo.KernelVersion,
			Arch:   hostInfo.KernelArch,
		}
		metric.Uptime = hostInfo.Uptime
	}

	// CPU
	if percent, err := cpu.Percent(time.Second, false); err == nil && len(percent) > 0 {
		metric.CPU.Percent = percent[0]
	}
	if count, err := cpu.Counts(true); err == nil {
		metric.CPU.Cores = count
	}
	if info, err := cpu.Info(); err == nil && len(info) > 0 {
		metric.CPU.Model = info[0].ModelName
	}

	// Memory
	if memInfo, err := mem.VirtualMemory(); err == nil {
		metric.Memory = models.MemoryInfo{
			Total:     memInfo.Total,
			Available: memInfo.Available,
			Used:      memInfo.Used,
			Percent:   memInfo.UsedPercent,
		}
	}

	// Swap
	if swapInfo, err := mem.SwapMemory(); err == nil {
		metric.Swap = models.SwapInfo{
			Total:   swapInfo.Total,
			Used:    swapInfo.Used,
			Percent: swapInfo.UsedPercent,
		}
	}

	// Disk
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

	// Network
	if netIO, err := net.IOCounters(false); err == nil && len(netIO) > 0 {
		metric.Network = models.NetworkInfo{
			BytesSent: netIO[0].BytesSent,
			BytesRecv: netIO[0].BytesRecv,
		}
	}

	// Load Average (Unix)
	if currentOS != "windows" {
		if loadAvg, err := load.Avg(); err == nil && loadAvg != nil {
			metric.Load = models.LoadInfo{
				Load1:  loadAvg.Load1,
				Load5:  loadAvg.Load5,
				Load15: loadAvg.Load15,
			}
		}
	}

	// Logs & Containers
	metric.Logs = collectSystemLogs(currentOS)
	metric.Containers = collectDockerContainers()

	return metric, nil
}

// Collect system logs
func collectSystemLogs(osName string) models.LogsInfo {
	logs := models.LogsInfo{}
	
	if osName == "windows" {
		logs.System = runPowerShell(`Get-EventLog -LogName System -Newest 50 | Out-String`)
		logs.Security = runPowerShell(`Get-EventLog -LogName Security -Newest 30 | Out-String`)
		return logs
	}

	// Linux/Darwin
	
	// System Logs
	sysLog, err := runCmdWithErr("journalctl", "-k", "-b", "-n", "50", "--no-pager", "-o", "cat")
	if err == nil && len(sysLog) > 10 {
		logs.System = sysLog
	} else {
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

	// Security Logs
	secLog, err := runCmdWithErr("journalctl", "_COMM=sshd", "-n", "50", "--no-pager", "-o", "cat") 
	if err == nil && len(secLog) > 10 {
		logs.Security = secLog
	} else {
		paths := []string{
			"/host/var/log/auth.log", "/var/log/auth.log",
			"/host/var/log/secure", "/var/log/secure",
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				logs.Security = runCmd("tail", "-n", "50", path)
				break
			}
		}
	}
	
	return logs
}

// Collect docker containers
func collectDockerContainers() []models.ContainerInfo {
	containers := []models.ContainerInfo{}

	// Skip if no docker socket
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		return containers
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Printf("Docker client error: %v", err)
		return containers
	}
	defer cli.Close()

	ctx := context.Background()
	containerList, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		log.Printf("Docker list error: %v", err)
		return containers
	}

	for _, c := range containerList {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		containers = append(containers, models.ContainerInfo{
			ID:      c.ID[:12],
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Created: c.Created,
			Logs:    collectContainerLogs(cli, c.ID),
		})
	}

	return containers
}

// Collect container logs
func collectContainerLogs(cli *client.Client, containerID string) string {
    ctx := context.Background()
    
    options := container.LogsOptions{
        ShowStdout: true,
        ShowStderr: true,
        Tail:       "100",
    }
    
    reader, err := cli.ContainerLogs(ctx, containerID, options)
    if err != nil {
        return ""
    }
    defer reader.Close()
    
    var buf bytes.Buffer
    _, _ = stdcopy.StdCopy(&buf, &buf, reader)
    
    return buf.String()
}

func runPowerShell(cmd string) string {
	out, _ := exec.Command("powershell", "-Command", cmd).CombinedOutput()
	return string(out)
}

func runCmdWithErr(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

func runCmd(name string, args ...string) string {
	out, _ := exec.Command(name, args...).CombinedOutput()
	return string(out)
}

// Get public IP with timeout
func getPublicIP() string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	ip, _ := io.ReadAll(resp.Body)
	return string(ip)
}


