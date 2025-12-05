package collector

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"time"

	"observex-agent/models"

	"github.com/docker/docker/api/types"
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

	// Docker Metrics
	dockerMetrics, err := collectDockerMetrics()
	if err == nil {
		metric.Docker = dockerMetrics
	}

	return metric, nil
}

func collectDockerMetrics() (*models.DockerInfo, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	dockerInfo := &models.DockerInfo{
		TotalContainers:   len(containers),
		RunningContainers: 0,
		StoppedContainers: 0,
		PausedContainers:  0,
		Containers:        make([]models.ContainerInfo, 0),
	}

	for _, c := range containers {
		switch c.State {
		case "running":
			dockerInfo.RunningContainers++
		case "exited":
			dockerInfo.StoppedContainers++
		case "paused":
			dockerInfo.PausedContainers++
		}

		containerInfo := models.ContainerInfo{
			ID:      c.ID[:12],
			Name:    c.Names[0][1:], // Remove leading '/'
			Image:   c.Image,
			State:   c.State,
			Status:  c.Status,
			Created: c.Created,
		}

		// Get container stats if running
		if c.State == "running" {
			stats, err := cli.ContainerStats(ctx, c.ID, false)
			if err == nil {
				var v types.StatsJSON
				if err := json.NewDecoder(stats.Body).Decode(&v); err == nil {
					// Calculate CPU percentage
					cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
					systemDelta := float64(v.CPUStats.SystemUsage - v.PreCPUStats.SystemUsage)
					cpuPercent := 0.0
					if systemDelta > 0 && cpuDelta > 0 {
						cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
					}

					// Memory
					memUsage := float64(v.MemoryStats.Usage)
					memLimit := float64(v.MemoryStats.Limit)
					memPercent := 0.0
					if memLimit > 0 {
						memPercent = (memUsage / memLimit) * 100.0
					}

					// Network
					var netRx, netTx uint64
					for _, net := range v.Networks {
						netRx += net.RxBytes
						netTx += net.TxBytes
					}

					// Block I/O
					var blockRead, blockWrite uint64
					for _, bio := range v.BlkioStats.IoServiceBytesRecursive {
						if bio.Op == "read" || bio.Op == "Read" {
							blockRead += bio.Value
						} else if bio.Op == "write" || bio.Op == "Write" {
							blockWrite += bio.Value
						}
					}

					containerInfo.CPUPercent = cpuPercent
					containerInfo.MemoryUsage = v.MemoryStats.Usage
					containerInfo.MemoryLimit = v.MemoryStats.Limit
					containerInfo.MemoryPercent = memPercent
					containerInfo.NetworkRx = netRx
					containerInfo.NetworkTx = netTx
					containerInfo.BlockRead = blockRead
					containerInfo.BlockWrite = blockWrite
				}
				stats.Body.Close()
			}
		}

		dockerInfo.Containers = append(dockerInfo.Containers, containerInfo)
	}

	return dockerInfo, nil
}