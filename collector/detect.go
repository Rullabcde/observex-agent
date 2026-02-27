package collector

import (
	"log"
	"os"
	"strings"
	"sync"
)

type Capabilities struct {
	HasDockerSocket bool
	HasHostPID      bool
	HasDBus         bool
	HasJournal      bool
	HasHostLogs     bool
}

var (
	caps     Capabilities
	capsOnce sync.Once
)

func DetectCapabilities() Capabilities {
	capsOnce.Do(func() {
		hasDBus := fileExists("/run/dbus/system_bus_socket")
		if hasDBus {
			// godbus/dbus defaults to /var/run/dbus/system_bus_socket, which doesn't exist 
			// in a scratch container since it lacks the /var/run -> /run symlink.
			os.Setenv("DBUS_SYSTEM_BUS_ADDRESS", "unix:path=/run/dbus/system_bus_socket")
		}

		caps = Capabilities{
			HasDockerSocket: fileExists("/var/run/docker.sock"),
			HasHostPID:      detectHostPID(),
			HasDBus:         hasDBus,
			HasJournal:      detectJournal(),
			HasHostLogs:     detectHostLogs(),
		}

		log.Println("╭─ Agent Capabilities ──────────────────────────────────────╮")
		logCap("Docker", caps.HasDockerSocket, "(container monitoring)")
		logCap("Host PID", caps.HasHostPID, "(process listing)")
		logCap("D-Bus", caps.HasDBus, "(systemd services)")
		logCap("Journal", caps.HasJournal, "(system logs via journalctl)")
		logCap("Host Logs", caps.HasHostLogs, "(log files (/var/log))")
		log.Println("╰───────────────────────────────────────────────────────────╯")
	})
	return caps
}

func logCap(name string, available bool, desc string) {
	icon := "✗"
	status := "unavailable"
	if available {
		icon = "✓"
		status = "enabled"
	}
	log.Printf("│ %s %-10s │ %-11s │ %-28s │", icon, name, status, desc)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func detectHostPID() bool {
	data, err := os.ReadFile("/proc/1/cmdline")
	if err != nil {
		return false
	}
	cmdline := strings.ReplaceAll(string(data), "\x00", " ")
	cmdline = strings.TrimSpace(strings.ToLower(cmdline))

	if strings.Contains(cmdline, "/agent") || strings.Contains(cmdline, "observex") {
		return false
	}
	return true
}

func detectJournal() bool {
	return fileExists("/run/log/journal")
}

func detectHostLogs() bool {
	paths := []string{
		"/host/var/log/syslog",
		"/host/var/log/messages",
		"/var/log/syslog",
		"/var/log/messages",
	}
	for _, path := range paths {
		if fileExists(path) {
			return true
		}
	}
	return false
}
