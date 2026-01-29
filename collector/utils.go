package collector

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Executes a PowerShell command
func runPowerShell(cmd string) string {
	out, _ := exec.Command("powershell", "-Command", cmd).CombinedOutput()
	return string(out)
}

// Executes a command and returns output + error
func runCmdWithErr(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

// Executes a command and returns output only
func runCmd(name string, args ...string) string {
	out, _ := exec.Command(name, args...).CombinedOutput()
	return string(out)
}

// Gets public IP with timeout
func getPublicIP() string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	ip, _ := io.ReadAll(resp.Body)
	return string(ip)
}

// Reads the last n lines from a file
func readLastLines(path string, n int) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return ""
	}
	filesize := stat.Size()

	// Read last 50KB which should covers 50 lines easily
	// Average log line < 200 bytes * 50 = 10KB
	const blockSize int64 = 50 * 1024

	startPos := filesize - blockSize
	if startPos < 0 {
		startPos = 0
	}

	if _, err := file.Seek(startPos, 0); err != nil {
		return ""
	}

	// Read to buffer
	bufLen := filesize - startPos
	buf := make([]byte, bufLen)
	if _, err := io.ReadFull(file, buf); err != nil {
		return ""
	}

	lines := strings.Split(string(buf), "\n")

	// Remove empty last line if exists (common in logs)
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return strings.Join(lines, "\n")
}
