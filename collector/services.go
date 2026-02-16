package collector

import (
	"bufio"
	"log"
	"os/exec"
	"runtime"
	"strings"

	"observex-agent/models"
)

// collectServices detects system services based on the OS.
func collectServices(currentOS string) []models.ServiceInfo {
	switch currentOS {
	case "linux":
		return collectLinuxServices()
	case "darwin":
		return collectDarwinServices()
	case "windows":
		return collectWindowsServices()
	default:
		log.Printf("Service collection not supported for OS: %s", currentOS)
		return nil
	}
}

// collectLinuxServices uses systemctl to list services.
// Falls back to /etc/init.d/ scanning for non-systemd systems.
func collectLinuxServices() []models.ServiceInfo {
	// Try systemctl first
	services := collectSystemdServices()
	if services != nil {
		return services
	}

	// Fallback: scan /etc/init.d/
	return collectInitDServices()
}

func collectSystemdServices() []models.ServiceInfo {
	cmd := exec.Command("systemctl", "list-units", "--type=service", "--all", "--no-pager", "--plain", "--no-legend")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("systemctl not available: %v", err)
		return nil
	}

	var services []models.ServiceInfo
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Format: UNIT LOAD ACTIVE SUB DESCRIPTION...
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		unit := fields[0]
		// active := fields[2] // active, inactive, failed
		sub := fields[3] // running, exited, dead, waiting, etc.

		// Get display name from the rest of the fields
		displayName := unit
		if len(fields) > 4 {
			displayName = strings.Join(fields[4:], " ")
		}

		// Clean up unit name (remove .service suffix)
		name := strings.TrimSuffix(unit, ".service")

		// Determine start type
		startType := getSystemdStartType(unit)

		// Normalize status
		status := normalizeLinuxStatus(sub)

		services = append(services, models.ServiceInfo{
			Name:        name,
			DisplayName: displayName,
			Status:      status,
			StartType:   startType,
		})
	}

	return services
}

func getSystemdStartType(unit string) string {
	cmd := exec.Command("systemctl", "is-enabled", unit)
	output, err := cmd.Output()
	if err != nil {
		// is-enabled returns non-zero for disabled/masked
		if exitErr, ok := err.(*exec.ExitError); ok {
			out := strings.TrimSpace(string(exitErr.Stderr))
			if out == "" {
				out = strings.TrimSpace(string(output))
			}
			return normalizeStartType(out)
		}
		return "unknown"
	}
	return normalizeStartType(strings.TrimSpace(string(output)))
}

func normalizeStartType(raw string) string {
	switch raw {
	case "enabled", "enabled-runtime":
		return "auto"
	case "disabled":
		return "disabled"
	case "masked", "masked-runtime":
		return "disabled"
	case "static", "indirect":
		return "manual"
	default:
		return "unknown"
	}
}

func normalizeLinuxStatus(sub string) string {
	switch sub {
	case "running":
		return "running"
	case "exited":
		return "stopped"
	case "dead":
		return "stopped"
	case "waiting", "start-pre", "start", "start-post":
		return "starting"
	case "failed":
		return "failed"
	default:
		return sub
	}
}

func collectInitDServices() []models.ServiceInfo {
	cmd := exec.Command("ls", "/etc/init.d/")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Failed to list /etc/init.d/: %v", err)
		return nil
	}

	var services []models.ServiceInfo
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if name == "" || name == "README" || name == "skeleton" {
			continue
		}

		// Try to check status via service command
		status := "unknown"
		statusCmd := exec.Command("service", name, "status")
		if err := statusCmd.Run(); err == nil {
			status = "running"
		} else {
			status = "stopped"
		}

		services = append(services, models.ServiceInfo{
			Name:        name,
			DisplayName: name,
			Status:      status,
			StartType:   "unknown",
		})
	}

	return services
}

// collectDarwinServices uses launchctl to list services on macOS.
func collectDarwinServices() []models.ServiceInfo {
	cmd := exec.Command("launchctl", "list")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Failed to run launchctl list: %v", err)
		return nil
	}

	var services []models.ServiceInfo
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	// Skip header line
	if scanner.Scan() {
		// discard header: PID Status Label
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Format: PID Status Label
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		pid := fields[0]
		label := fields[2]

		status := "stopped"
		if pid != "-" {
			status = "running"
		}

		services = append(services, models.ServiceInfo{
			Name:        label,
			DisplayName: label,
			Status:      status,
			StartType:   "auto", // launchd services are typically auto
		})
	}

	return services
}

// collectWindowsServices uses PowerShell to enumerate Windows services.
func collectWindowsServices() []models.ServiceInfo {
	if runtime.GOOS != "windows" {
		return nil
	}

	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"Get-Service | Select-Object Name,DisplayName,Status,StartType | ConvertTo-Csv -NoTypeInformation")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Failed to get Windows services: %v", err)
		return nil
	}

	var services []models.ServiceInfo
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	// Skip CSV header
	if scanner.Scan() {
		// discard: "Name","DisplayName","Status","StartType"
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := parseCSVLine(line)
		if len(fields) < 4 {
			continue
		}

		name := fields[0]
		displayName := fields[1]
		rawStatus := fields[2]
		rawStartType := fields[3]

		status := normalizeWindowsStatus(rawStatus)
		startType := normalizeWindowsStartType(rawStartType)

		services = append(services, models.ServiceInfo{
			Name:        name,
			DisplayName: displayName,
			Status:      status,
			StartType:   startType,
		})
	}

	return services
}

func normalizeWindowsStatus(raw string) string {
	switch strings.ToLower(raw) {
	case "running", "4":
		return "running"
	case "stopped", "1":
		return "stopped"
	case "paused", "7":
		return "paused"
	case "startpending", "2":
		return "starting"
	case "stoppending", "3":
		return "stopping"
	default:
		return "unknown"
	}
}

func normalizeWindowsStartType(raw string) string {
	switch strings.ToLower(raw) {
	case "automatic", "2":
		return "auto"
	case "manual", "3":
		return "manual"
	case "disabled", "4":
		return "disabled"
	default:
		return "unknown"
	}
}

// parseCSVLine parses a simple CSV line, handling quoted fields.
func parseCSVLine(line string) []string {
	var fields []string
	var field strings.Builder
	inQuotes := false

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '"':
			if inQuotes && i+1 < len(line) && line[i+1] == '"' {
				field.WriteByte('"')
				i++ // skip escaped quote
			} else {
				inQuotes = !inQuotes
			}
		case c == ',' && !inQuotes:
			fields = append(fields, field.String())
			field.Reset()
		default:
			field.WriteByte(c)
		}
	}
	fields = append(fields, field.String())

	return fields
}
