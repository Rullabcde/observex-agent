package collector

import (
	"bufio"
	"context"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/uptime-id/agent/models"

	"github.com/coreos/go-systemd/v22/dbus"
)

var dbusErrorOnce sync.Once

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
		dbusErrorOnce.Do(func() {
			log.Printf("D-Bus unavailable (systemd services disabled): %v", err)
		})
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
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ls", "/etc/init.d/")
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
		statusCtx, statusCancel := context.WithTimeout(context.Background(), 5*time.Second)
		statusCmd := exec.CommandContext(statusCtx, "service", name, "status")
		if err := statusCmd.Run(); err == nil {
			status = "running"
		} else {
			status = "stopped"
		}
		statusCancel()

		services = append(services, models.ServiceInfo{
			Name:        name,
			DisplayName: name,
			Status:      status,
			StartType:   "unknown",
		})
	}

	return services
}

func collectDarwinServices() []models.ServiceInfo {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "launchctl", "list")
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

func collectWindowsServices() []models.ServiceInfo {
	if runtime.GOOS != "windows" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
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
