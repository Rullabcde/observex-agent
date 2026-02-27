package collector

import (
	"os"
	"runtime"
	"time"

	"github.com/uptime-id/agent/models"
)

func CollectMetrics() (*models.Metric, error) {
	timestamp := time.Now()
	currentOS := runtime.GOOS
	caps := DetectCapabilities()

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

	// Core metrics
	collectSystemInfo(metric)
	collectCPUInfo(metric)
	collectMemoryInfo(metric)
	collectDiskInfo(metric, currentOS)
	collectLoadInfo(metric, currentOS)

	// Network
	metric.Network = collectNetworkInfo()

	// Latency
	metric.Latency = collectLatency()

	// Optional: Docker containers (needs /var/run/docker.sock)
	if caps.HasDockerSocket {
		metric.Containers = collectDockerContainers()
	}

	// Optional: System logs (needs journal or /var/log mount)
	if caps.HasJournal || caps.HasHostLogs {
		metric.Logs = collectSystemLogs(currentOS)
	}

	// Optional: Host processes (needs pid: host)
	if caps.HasHostPID {
		metric.Processes = collectTopProcesses()
	}

	// Optional: Systemd services (needs D-Bus socket)
	if caps.HasDBus {
		metric.Services = collectServices(currentOS)
	}

	return metric, nil
}
