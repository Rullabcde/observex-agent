package collector

import (
	"os"
	"runtime"
	"time"

	"observex-agent/models"
)

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

	// System & Resources
	collectSystemInfo(metric)
	collectCPUInfo(metric)
	collectMemoryInfo(metric)
	collectDiskInfo(metric, currentOS)
	collectLoadInfo(metric, currentOS)

	// Network
	metric.Network = collectNetworkInfo()

	// External/High-latency checks
	metric.Logs = collectSystemLogs(currentOS)
	metric.Containers = collectDockerContainers()
	metric.Latency = collectLatency()
	metric.Processes = collectTopProcesses()

	return metric, nil
}
