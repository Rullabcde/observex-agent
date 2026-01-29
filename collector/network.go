package collector

import (
	"net"
	"time"

	"observex-agent/models"

	gopsnet "github.com/shirou/gopsutil/v3/net"
)

// Gathers network I/O stats
func collectNetworkInfo() models.NetworkInfo {
	if netIO, err := gopsnet.IOCounters(false); err == nil && len(netIO) > 0 {
		return models.NetworkInfo{
			BytesSent: netIO[0].BytesSent,
			BytesRecv: netIO[0].BytesRecv,
		}
	}
	return models.NetworkInfo{}
}

// Gather latency to public DNS servers
func collectLatency() []models.LatencyInfo {
	targets := []string{"8.8.8.8:53", "1.1.1.1:53"}
	results := make([]models.LatencyInfo, 0, len(targets))

	for _, targetAddr := range targets {
		host, _, _ := net.SplitHostPort(targetAddr)
		if host == "" {
			host = targetAddr
		}

		info := models.LatencyInfo{
			Target:  host,
			Success: false,
		}

		start := time.Now()
		conn, err := net.DialTimeout("tcp", targetAddr, 2*time.Second)
		if err == nil {
			conn.Close()
			latency := time.Since(start).Seconds() * 1000 // ms
			info.Latency = latency
			info.Success = true
		} else {
		}

		results = append(results, info)
	}

	return results
}
