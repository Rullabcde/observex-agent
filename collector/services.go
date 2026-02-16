package collector

import (
	"bufio"
	"context"
	"log"
	"os/exec"
	"runtime"
	"strings"

	"observex-agent/models"

	"github.com/coreos/go-systemd/v22/dbus"
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

// collectLinuxServices uses systemctl (via D-Bus) to list services.
// Falls back to /etc/init.d/ scanning for non-systemd systems.
func collectLinuxServices() []models.ServiceInfo {
	services := collectSystemdServices()
	if services != nil {
		return services
	}

	return collectInitDServices()
}

func collectSystemdServices() []models.ServiceInfo {
    ctx := context.Background()
	conn, err := dbus.NewWithContext(ctx)
	if err != nil {
		log.Printf("Failed to connect to systemd bus: %v", err)
		return nil
	}
	defer conn.Close()

	units, err := conn.ListUnitsContext(ctx)
	if err != nil {
		log.Printf("Failed to list systemd units: %v", err)
		return nil
	}

	var services []models.ServiceInfo

	for _, unit := range units {
		if !strings.HasSuffix(unit.Name, ".service") {
			continue
		}

		name := strings.TrimSuffix(unit.Name, ".service")
		displayName := unit.Description
		if displayName == "" {
			displayName = name
		}

		startType := "unknown"
		if prop, err := conn.GetUnitPropertyContext(ctx, unit.Name, "UnitFileState"); err == nil && prop != nil {
			if val, ok := prop.Value.Value().(string); ok {
				startType = normalizeStartType(val)
			}
		}

		status := normalizeLinuxStatus(unit.SubState)

		services = append(services, models.ServiceInfo{
			Name:        name,
			DisplayName: displayName,
			Status:      status,
			StartType:   startType,
		})
	}

	return services
}

func normalizeStartType(raw string) string {
	switch raw {
	case "enabled", "enabled-runtime", "generated", "static", "indirect":
		return "auto"
	case "disabled", "masked", "masked-runtime":
		return "disabled"
	default:
		return "manual"
	}
}

func normalizeLinuxStatus(sub string) string {
	switch sub {
	case "running":
		return "running"
	case "exited", "dead":
		return "stopped"
	case "waiting", "start-pre", "start", "start-post", "reloading":
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

	if scanner.Scan() {
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

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
			StartType:   "auto",
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

	if scanner.Scan() {
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
				i++
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
