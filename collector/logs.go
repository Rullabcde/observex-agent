package collector

import (
	"os"

	"observex-agent/models"
)

// Gathers system logs based on OS
func collectSystemLogs(osName string) models.LogsInfo {
	logs := models.LogsInfo{}

	if osName == "windows" {
		logs.System = runPowerShell(`Get-EventLog -LogName System -Newest 50 | Out-String`)
		logs.Security = runPowerShell(`Get-EventLog -LogName Security -Newest 30 | Out-String`)
		return logs
	}

	// System Logs
	sysLog, err := runCmdWithErr("journalctl", "-k", "-b", "-n", "50", "--no-pager", "-o", "cat")
	if err == nil && len(sysLog) > 10 {
		logs.System = sysLog
	} else {
		paths := []string{
			"/host/var/log/syslog", "/var/log/syslog",
			"/host/var/log/messages", "/var/log/messages",
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				logs.System = readLastLines(path, 50)
				break
			}
		}
	}

	// Security Logs
	secLog, err := runCmdWithErr("journalctl", "_COMM=sshd", "-n", "50", "--no-pager", "-o", "cat")
	if err == nil && len(secLog) > 10 {
		logs.Security = secLog
	} else {
		paths := []string{
			"/host/var/log/auth.log", "/var/log/auth.log",
			"/host/var/log/secure", "/var/log/secure",
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				logs.Security = readLastLines(path, 50)
				break
			}
		}
	}

	return logs
}
