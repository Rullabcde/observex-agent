package collector

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/uptime-id/agent/models"

	"bytes"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

const maxTotalContainerLogSize = 500 * 1024
const maxPerContainerLogSize = 50 * 1024

func collectDockerContainers() []models.ContainerInfo {
	containers := []models.ContainerInfo{}
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		return containers
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Printf("Docker client error: %v", err)
		return containers
	}
	defer cli.Close()

	ctx := context.Background()
	containerList, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		log.Printf("Docker list error: %v", err)
		return containers
	}

	totalLogSize := 0

	for _, c := range containerList {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		logs := ""
		if totalLogSize < maxTotalContainerLogSize {
			logs = collectContainerLogs(cli, c.ID)
			if len(logs) > maxPerContainerLogSize {
				logs = logs[:maxPerContainerLogSize] + "\n[TRUNCATED]"
			}
			totalLogSize += len(logs)
		}

		containers = append(containers, models.ContainerInfo{
			ID:      c.ID[:12],
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Created: c.Created,
			Logs:    logs,
		})
	}

	return containers
}

func collectContainerLogs(cli *client.Client, containerID string) string {
	ctx := context.Background()

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "50",
	}

	reader, err := cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return ""
	}
	defer reader.Close()

	var buf bytes.Buffer
	_, _ = stdcopy.StdCopy(&buf, &buf, reader)

	return buf.String()
}
