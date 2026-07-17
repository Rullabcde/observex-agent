package collector

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/uptime-id/agent/models"
)

// connSnapshot stores cumulative bytes for a network connection identified by inode.
type connSnapshot struct {
	pid      int
	name     string
	remoteIP string
	remotePort int
	protocol string
	rxBytes  uint64
	txBytes  uint64
}

var (
	prevNetConns     map[string]connSnapshot
	prevNetConnsMu   sync.Mutex
	prevNetConnsTime int64
)

func collectNetworkConnections() []models.NetworkConnectionInfo {
	conns := parseSsConnections()
	if len(conns) == 0 {
		return nil
	}

	// Aggregate by PID: sum rx/tx bytes
	type pidNet struct {
		pid        int
		name       string
		rxBytes    uint64
		txBytes    uint64
		remoteAddr string
		remotePort int
		protocol   string
	}

	byPid := make(map[int]*pidNet)
	for _, c := range conns {
		if _, exists := byPid[c.pid]; !exists {
			byPid[c.pid] = &pidNet{
				pid:        c.pid,
				name:       c.name,
				remoteAddr: c.remoteIP,
				remotePort: c.remotePort,
				protocol:   c.protocol,
			}
		}
		p := byPid[c.pid]
		p.rxBytes += c.rxBytes
		p.txBytes += c.txBytes
	}

	// Calculate rates using delta from previous snapshot
	now := timeNowUnix()
	prevNetConnsMu.Lock()
	prev := prevNetConns
	prevTime := prevNetConnsTime

	// Save current snapshot for next interval
	nextPrev := make(map[string]connSnapshot, len(conns))
	for _, c := range conns {
		key := fmt.Sprintf("%d:%s:%s:%d", c.pid, c.protocol, c.remoteIP, c.remotePort)
		nextPrev[key] = c
	}
	prevNetConns = nextPrev
	prevNetConnsTime = now
	prevNetConnsMu.Unlock()

	interval := float64(now - prevTime)
	if interval <= 0 {
		interval = 5 // default
	}

	var results []models.NetworkConnectionInfo
	for pid, p := range byPid {
		rxRate := 0.0
		txRate := 0.0

		if prev != nil && prevTime > 0 {
			// Find matching previous entry for this pid
			for _, prevSnap := range prev {
				if prevSnap.pid == pid {
					if p.rxBytes >= prevSnap.rxBytes {
						rxRate = float64(p.rxBytes-prevSnap.rxBytes) / interval
					}
					if p.txBytes >= prevSnap.txBytes {
						txRate = float64(p.txBytes-prevSnap.txBytes) / interval
					}
					break
				}
			}
		}

		results = append(results, models.NetworkConnectionInfo{
			PID:        p.pid,
			Name:       p.name,
			RemoteIP:   p.remoteAddr,
			RemotePort: p.remotePort,
			Protocol:   p.protocol,
			RxRate:     rxRate,
			TxRate:     txRate,
		})
	}

	// Sort by total rate descending
	sort.Slice(results, func(i, j int) bool {
		return (results[i].RxRate + results[i].TxRate) > (results[j].RxRate + results[j].TxRate)
	})

	// Limit to top 10
	if len(results) > 10 {
		results = results[:10]
	}

	return results
}

// timeNowUnix returns current unix timestamp in seconds.
func timeNowUnix() int64 {
	return time.Now().Unix()
}

// ssConnection holds parsed data from ss command.
type ssConnection struct {
	pid        int
	name       string
	remoteIP   string
	remotePort int
	protocol   string
	rxBytes    uint64
	txBytes    uint64
}

func parseSsConnections() []ssConnection {
	// Try reading /proc/net/tcp first (more reliable in containers)
	return parseProcNetTcp()
}

// parseProcNetTcp reads /proc/net/tcp and maps sockets to processes via /proc/[pid]/net/tcp
func parseProcNetTcp() []ssConnection {
	// Read all socket inodes from /proc/net/tcp
	tcpEntries, err := readProcNetEntries("/proc/net/tcp", "tcp")
	if err != nil {
		return nil
	}
	tcp6Entries, err := readProcNetEntries("/proc/net/tcp6", "tcp6")
	if err != nil {
		if len(tcpEntries) == 0 {
			return nil
		}
	}
	allEntries := append(tcpEntries, tcp6Entries...)

	if len(allEntries) == 0 {
		return nil
	}

	// Build inode -> (remote_ip, remote_port, protocol) mapping
	type entryInfo struct {
		remoteIP   string
		remotePort int
		protocol   string
	}
	inodeMap := make(map[string]entryInfo)
	for _, e := range allEntries {
		inodeMap[e.inode] = entryInfo{
			remoteIP:   e.remoteIP,
			remotePort: e.remotePort,
			protocol:   e.protocol,
		}
	}

	// Map inodes to PIDs by scanning /proc/[pid]/fd
	pidInodes := mapInodesToPIDs()
	if len(pidInodes) == 0 {
		return nil
	}

	var results []ssConnection
	for inode, info := range inodeMap {
		pid, name, ok := pidInodes[inode]
		if !ok || pid <= 0 {
			continue
		}
		results = append(results, ssConnection{
			pid:        pid,
			name:       name,
			remoteIP:   info.remoteIP,
			remotePort: info.remotePort,
			protocol:   info.protocol,
		})
	}

	return results
}

type procNetEntry struct {
	inode      string
	remoteIP   string
	remotePort int
	protocol   string
}

func readProcNetEntries(path string, protocol string) ([]procNetEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []procNetEntry
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum == 1 {
			continue // skip header
		}
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		// fields[2] = remote address (hex IP:port)
		remoteAddr := fields[2]
		parts := strings.SplitN(remoteAddr, ":", 2)
		if len(parts) != 2 {
			continue
		}
		remoteIPHex := parts[0]
		remotePortHex := parts[1]

		port, err := strconv.ParseUint(remotePortHex, 16, 16)
		if err != nil {
			continue
		}

		ip := hexToIP(remoteIPHex)
		if ip == "" {
			continue
		}

		// Skip connections to 0.0.0.0, 127.0.0.1, or with 0 remote port (listening sockets)
		if port == 0 {
			continue
		}

		inode := fields[9]
		entries = append(entries, procNetEntry{
			inode:      inode,
			remoteIP:   ip,
			remotePort: int(port),
			protocol:   protocol,
		})
	}

	return entries, nil
}

// hexToIP converts a hex IP address from /proc/net/tcp to dotted notation.
func hexToIP(hexIP string) string {
	if len(hexIP) != 8 && len(hexIP) != 32 {
		return ""
	}

	if len(hexIP) == 8 {
		// IPv4
		b := make([]byte, 4)
		for i := 0; i < 4; i++ {
			val, err := strconv.ParseUint(hexIP[i*2:i*2+2], 16, 8)
			if err != nil {
				return ""
			}
			b[i] = byte(val)
		}
		// /proc/net/tcp stores IP in network byte order (little-endian on x86)
		return net.IPv4(b[3], b[2], b[1], b[0]).String()
	}

	// IPv6 — simplified, just return hex
	b := make([]byte, 16)
	for i := 0; i < 16; i++ {
		val, err := strconv.ParseUint(hexIP[i*2:i*2+2], 16, 8)
		if err != nil {
			return ""
		}
		b[i] = byte(val)
	}
	ip := net.IP(b)
	return ip.String()
}

// pidInodeResult holds the result of mapping an inode to a pid.
type pidInodeResult struct {
	pid  int
	name string
}

// mapInodesToPIDs scans /proc/[pid]/fd for socket symlinks and returns a map of inode -> (pid, name).
func mapInodesToPIDs() map[string]pidInodeResult {
	procDir, err := os.Open("/proc")
	if err != nil {
		return nil
	}
	defer procDir.Close()

	dirs, err := procDir.Readdirnames(-1)
	if err != nil {
		return nil
	}

	result := make(map[string]pidInodeResult)

	for _, dir := range dirs {
		pid, err := strconv.Atoi(dir)
		if err != nil || pid <= 0 {
			continue
		}

		// Get process name from /proc/[pid]/comm
		comm, err := os.ReadFile(filepath.Join("/proc", dir, "comm"))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(comm))

		fdDir := filepath.Join("/proc", dir, "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}

		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			// socket:[123456]
			if strings.HasPrefix(link, "socket:[") && strings.HasSuffix(link, "]") {
				inode := link[8 : len(link)-1]
				result[inode] = pidInodeResult{pid: pid, name: name}
			}
		}
	}

	return result
}
