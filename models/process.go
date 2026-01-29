package models

type ProcessInfo struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	User       string  `json:"user"`        // New: User owner
	Status     string  `json:"status"`      // New: Process state (R, S, Z, etc.)
	CPU        float64 `json:"cpu"`         // % CPU
	Memory     float64 `json:"memory"`      // % Memory
	ResMemory  uint64  `json:"res_memory"`  // New: Resident Memory (bytes)
	VirtMemory uint64  `json:"virt_memory"` // New: Virtual Memory (bytes)
	Time       string  `json:"time"`        // New: Exported as string (e.g. "0:05.12")
	Command    string  `json:"command"`
}
