package collector

import (
	"log"
	"sort"

	"observex-agent/models"

	"github.com/shirou/gopsutil/v3/process"
)

// Gets top 10 processes by CPU and memory usage
func collectTopProcesses() []models.ProcessInfo {
	procs, err := process.Processes()
	if err != nil {
		log.Printf("Failed to get processes: %v", err)
		return nil
	}

	type procData struct {
		pid     int32
		name    string
		cpu     float64
		memory  float32
		command string
	}

	var procList []procData

	for _, p := range procs {
		name, err := p.Name()
		if err != nil {
			continue
		}

		cpu, err := p.CPUPercent()
		if err != nil {
			cpu = 0
		}

		mem, err := p.MemoryPercent()
		if err != nil {
			mem = 0
		}

		cmdline, _ := p.Cmdline()
		if cmdline == "" {
			cmdline = name
		}

		if len(cmdline) > 200 {
			cmdline = cmdline[:200] + "..."
		}

		procList = append(procList, procData{
			pid:     p.Pid,
			name:    name,
			cpu:     cpu,
			memory:  mem,
			command: cmdline,
		})
	}

	sort.Slice(procList, func(i, j int) bool {
		scoreI := procList[i].cpu + float64(procList[i].memory)
		scoreJ := procList[j].cpu + float64(procList[j].memory)
		return scoreI > scoreJ
	})

	// Get top 10
	limit := 10
	if len(procList) < limit {
		limit = len(procList)
	}

	results := make([]models.ProcessInfo, 0, limit)
	for _, p := range procList[:limit] {
		results = append(results, models.ProcessInfo{
			PID:     int(p.pid),
			Name:    p.name,
			CPU:     p.cpu,
			Memory:  float64(p.memory),
			Command: p.command,
		})
	}

	return results
}
