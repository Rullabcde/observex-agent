package models

type SystemInfo struct {
	OS     string `json:"os"`
	Kernel string `json:"kernel"`
	Arch   string `json:"arch"`
}

type CPUInfo struct {
	Percent float64 `json:"percent"`
	Model   string  `json:"model"`
	Cores   int     `json:"cores"`
}

type MemoryInfo struct {
	Total     uint64  `json:"total"`
	Available uint64  `json:"available"`
	Used      uint64  `json:"used"`
	Percent   float64 `json:"percent"`
}

type SwapInfo struct {
	Total   uint64  `json:"total"`
	Used    uint64  `json:"used"`
	Percent float64 `json:"percent"`
}

type DiskInfo struct {
	Total      uint64  `json:"total"`
	Free       uint64  `json:"free"`
	Used       uint64  `json:"used"`
	Percent    float64 `json:"percent"`
	ReadBytes  uint64  `json:"readBytes"`
	WriteBytes uint64  `json:"writeBytes"`
}

type LoadInfo struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}


