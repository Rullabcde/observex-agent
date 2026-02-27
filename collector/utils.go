package collector

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const cmdTimeout = 10 * time.Second

func runPowerShell(cmd string) string {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, _ := exec.CommandContext(ctx, "powershell", "-Command", cmd).CombinedOutput()
	return string(out)
}

func runCmdWithErr(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}

func getPublicIP() string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	limitedReader := io.LimitReader(resp.Body, 64)
	ipBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return ""
	}

	ipStr := strings.TrimSpace(string(ipBytes))

	if net.ParseIP(ipStr) == nil {
		return ""
	}

	return ipStr
}

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

	const blockSize int64 = 50 * 1024

	startPos := filesize - blockSize
	if startPos < 0 {
		startPos = 0
	}

	if _, err := file.Seek(startPos, 0); err != nil {
		return ""
	}

	bufLen := filesize - startPos
	buf := make([]byte, bufLen)
	if _, err := io.ReadFull(file, buf); err != nil {
		return ""
	}

	lines := strings.Split(string(buf), "\n")

	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return strings.Join(lines, "\n")
}
