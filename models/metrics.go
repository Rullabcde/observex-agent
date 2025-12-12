package models

import "time"

// SystemInfo holds OS details
type SystemInfo struct {
	OS     string `json:"os"`
	Kernel string `json:"kernel"`
	Arch   string `json:"arch"`
}

// CPUInfo holds CPU stats
type CPUInfo struct {
	Percent float64 `json:"percent"`
	Model   string  `json:"model"`
	Cores   int     `json:"cores"`
}

// MemoryInfo holds RAM stats
type MemoryInfo struct {
	Total     uint64  `json:"total"`
	Available uint64  `json:"available"`
	Used      uint64  `json:"used"`
	Percent   float64 `json:"percent"`
}

// SwapInfo holds Swap stats
type SwapInfo struct {
	Total   uint64  `json:"total"`
	Used    uint64  `json:"used"`
	Percent float64 `json:"percent"`
}

// DiskInfo holds Disk usage stats
type DiskInfo struct {
	Total      uint64  `json:"total"`
	Free       uint64  `json:"free"`
	Used       uint64  `json:"used"`
	Percent    float64 `json:"percent"`
	ReadBytes  uint64  `json:"readBytes"`
	WriteBytes uint64  `json:"writeBytes"`
}

// NetworkInfo holds Network I/O stats
type NetworkInfo struct {
	BytesSent uint64 `json:"bytesSent"`
	BytesRecv uint64 `json:"bytesRecv"`
}

// LoadInfo holds Load Average stats (Linux only)
type LoadInfo struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// LogsInfo holds system logs
type LogsInfo struct {
	System   string `json:"system"`
	Security string `json:"security"`
}

// ContainerInfo holds Docker container details
type ContainerInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	State   string `json:"state"`
	Created int64  `json:"created"`
}

// Metric is the main metric data structure collection
type Metric struct {
	Timestamp time.Time   `json:"timestamp"`
	Hostname  string      `json:"hostname"`
	OS        string      `json:"os"`
	System    SystemInfo  `json:"system"`
	Uptime    uint64      `json:"uptime"`
	CPU       CPUInfo     `json:"cpu"`
	Memory    MemoryInfo  `json:"memory"`
	Swap      SwapInfo    `json:"swap"`
	Disk      DiskInfo    `json:"disk"`
	Network   NetworkInfo `json:"network"`
	Load       LoadInfo        `json:"load"`
	Logs       LogsInfo        `json:"logs"`
	Containers []ContainerInfo `json:"containers,omitempty"`
}

// MetricPayload is the flat payload sent to API
type MetricPayload struct {
	CPU         float64   `json:"cpu"`
	CPUModel    string    `json:"cpuModel"`
	CPUCores    int       `json:"cpuCores"`
	Memory      float64   `json:"memory"`
	MemoryUsed  float64   `json:"memoryUsed"`
	MemoryTotal float64   `json:"memoryTotal"`
	Swap        float64   `json:"swap"`
	SwapUsed    float64   `json:"swapUsed"`
	SwapTotal   float64   `json:"swapTotal"`
	Disk        float64   `json:"disk"`
	DiskUsed    float64   `json:"diskUsed"`
	DiskTotal   float64   `json:"diskTotal"`
	DiskRead    float64   `json:"diskRead"`
	DiskWrite   float64   `json:"diskWrite"`
	NetworkIn   float64   `json:"networkIn"`
	NetworkOut  float64   `json:"networkOut"`
	Load1       float64   `json:"load1"`
	Load5       float64   `json:"load5"`
	Load15      float64   `json:"load15"`
	Uptime      float64   `json:"uptime"`
	Hostname    string    `json:"hostname"`
	OS          string    `json:"os"`
	Kernel      string          `json:"kernel"`
	Arch        string          `json:"arch"`
	Logs        *LogsInfo       `json:"logs,omitempty"`
	Containers  []ContainerInfo `json:"containers,omitempty"`
}

// ToPayload converts Metric to MetricPayload
func (m *Metric) ToPayload() *MetricPayload {
	return &MetricPayload{
		CPU:         m.CPU.Percent,
		CPUModel:    m.CPU.Model,
		CPUCores:    m.CPU.Cores,
		Memory:      m.Memory.Percent,
		MemoryUsed:  float64(m.Memory.Used),
		MemoryTotal: float64(m.Memory.Total),
		Swap:        m.Swap.Percent,
		SwapUsed:    float64(m.Swap.Used),
		SwapTotal:   float64(m.Swap.Total),
		Disk:        m.Disk.Percent,
		DiskUsed:    float64(m.Disk.Used),
		DiskTotal:   float64(m.Disk.Total),
		DiskRead:    float64(m.Disk.ReadBytes),
		DiskWrite:   float64(m.Disk.WriteBytes),
		NetworkIn:   float64(m.Network.BytesRecv),
		NetworkOut:  float64(m.Network.BytesSent),
		Load1:       m.Load.Load1,
		Load5:       m.Load.Load5,
		Load15:      m.Load.Load15,
		Uptime:      float64(m.Uptime),
		Hostname:    m.Hostname,
		OS:          m.System.OS,
		Kernel:     m.System.Kernel,
		Arch:       m.System.Arch,
		Logs:       &m.Logs,
		Containers: m.Containers,
	}
}
