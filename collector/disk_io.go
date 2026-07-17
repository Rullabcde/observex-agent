package collector

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/uptime-id/agent/models"

	"github.com/shirou/gopsutil/v3/process"
)

// diskIOSnapshot stores cumulative I/O bytes for a process.
type diskIOSnapshot struct {
	readBytes  uint64
	writeBytes uint64
}

var (
	prevDiskIO     map[int32]diskIOSnapshot
	prevDiskIOMu   sync.Mutex
	prevDiskIOTime int64
)

func collectDiskIO() []models.DiskIOInfo {
	now := time.Now().Unix()

	procs, err := process.Processes()
	if err != nil {
		return nil
	}

	currentIO := make(map[int32]diskIOSnapshot)
	var procsWithIO []struct {
		pid        int32
		name       string
		readBytes  uint64
		writeBytes uint64
	}

	for _, p := range procs {
		ioPath := fmt.Sprintf("/proc/%d/io", p.Pid)
		readB, writeB, ok := readProcIO(ioPath)
		if !ok {
			continue
		}

		name, err := p.Name()
		if err != nil {
			continue
		}

		currentIO[p.Pid] = diskIOSnapshot{readBytes: readB, writeBytes: writeB}
		procsWithIO = append(procsWithIO, struct {
			pid        int32
			name       string
			readBytes  uint64
			writeBytes uint64
		}{pid: p.Pid, name: name, readBytes: readB, writeBytes: writeB})
	}

	if len(procsWithIO) == 0 {
		return nil
	}

	// Calculate delta rates
	prevDiskIOMu.Lock()
	prev := prevDiskIO
	prevTime := prevDiskIOTime
	prevDiskIO = currentIO
	prevDiskIOTime = now
	prevDiskIOMu.Unlock()

	interval := float64(now - prevTime)
	if interval <= 0 {
		interval = 5
	}

	var results []models.DiskIOInfo
	for _, p := range procsWithIO {
		readRate := 0.0
		writeRate := 0.0

		if prev != nil && prevTime > 0 {
			if prevSnap, ok := prev[p.pid]; ok {
				if p.readBytes >= prevSnap.readBytes {
					readRate = float64(p.readBytes-prevSnap.readBytes) / interval
				}
				if p.writeBytes >= prevSnap.writeBytes {
					writeRate = float64(p.writeBytes-prevSnap.writeBytes) / interval
				}
			}
		}

		// Skip processes with zero activity
		if readRate == 0 && writeRate == 0 {
			continue
		}

		results = append(results, models.DiskIOInfo{
			PID:       int(p.pid),
			Name:      p.name,
			ReadRate:  readRate,
			WriteRate: writeRate,
		})
	}

	// Sort by total I/O rate descending
	sort.Slice(results, func(i, j int) bool {
		return (results[i].ReadRate + results[i].WriteRate) > (results[j].ReadRate + results[j].WriteRate)
	})

	// Limit to top 10
	if len(results) > 10 {
		results = results[:10]
	}

	return results
}

// readProcIO reads read_bytes and write_bytes from /proc/[pid]/io.
func readProcIO(path string) (readBytes, writeBytes uint64, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "read_bytes":
			readBytes, _ = strconv.ParseUint(val, 10, 64)
		case "write_bytes":
			writeBytes, _ = strconv.ParseUint(val, 10, 64)
		}
	}

	return readBytes, writeBytes, true
}
