package models

type NetworkInfo struct {
	BytesSent uint64 `json:"bytesSent"`
	BytesRecv uint64 `json:"bytesRecv"`
}

type LatencyInfo struct {
	Target  string  `json:"target"`
	Latency float64 `json:"latency"`
	Success bool    `json:"success"`
}
