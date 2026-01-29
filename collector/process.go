package collector

import (
	"fmt"
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
		pid      int32
		name     string
		user     string
		status   string
		cpu      float64
		memory   float32
		resMem   uint64
		virtMem  uint64
		timeStr  string
		command  string
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

		// Get User
		user, err := p.Username()
		if err != nil {
			user = ""
		}

		// Get Status
		statusVals, err := p.Status()
		status := ""
		if err == nil && len(statusVals) > 0 {
			status = statusVals[0]
		}

		// Get Memory Info (RES/VIRT)
		resMem := uint64(0)
		virtMem := uint64(0)
		memInfo, err := p.MemoryInfo()
		if err == nil {
			resMem = memInfo.RSS
			virtMem = memInfo.VMS
		}

		// Get Time
		timeStr := "0:00.00"
		times, err := p.Times()
		if err == nil {
			totalSecs := times.User + times.System
			mins := int(totalSecs / 60)
			secs := totalSecs - float64(mins*60)
			timeStr = fmt.Sprintf("%d:%05.2f", mins, secs)
		}

		// Get Command
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
			user:    user,
			status:  status,
			cpu:     cpu,
			memory:  mem,
			resMem:  resMem,
			virtMem: virtMem,
			timeStr: timeStr,
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
			PID:        int(p.pid),
			Name:       p.name,
			User:       p.user,
			Status:     p.status,
			CPU:        p.cpu,
			Memory:     float64(p.memory),
			ResMemory:  p.resMem,
			VirtMemory: p.virtMem,
			Time:       p.timeStr,
			Command:    p.command,
		})
	}

	return results
}
