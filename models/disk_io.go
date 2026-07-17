package models

type DiskIOInfo struct {
	PID       int     `json:"pid"`
	Name      string  `json:"name"`
	ReadRate  float64 `json:"readRate"`
	WriteRate float64 `json:"writeRate"`
}
