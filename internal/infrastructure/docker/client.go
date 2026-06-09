package docker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/domain/live"
)

type ClientFactory struct {
	tlsConfig  *tls.Config
	authBase64 string
	liveRefStr string
	dbRefStr   string // только Postgres
}

type ClientFactoryConfig struct {
	TLSConfig  *tls.Config
	AuthBase64 string
	LiveRefStr string
	DBRefStr   string
}

func NewClientFactory(cfg ClientFactoryConfig) *ClientFactory {
	return &ClientFactory{
		tlsConfig:  cfg.TLSConfig,
		authBase64: cfg.AuthBase64,
		liveRefStr: cfg.LiveRefStr,
		dbRefStr:   cfg.DBRefStr,
	}
}

func (f *ClientFactory) NewClient(addr string) (*Client, error) {
	const op = "docker new client"
	transport := &http.Transport{TLSClientConfig: f.tlsConfig}
	httpClient := &http.Client{Transport: transport}

	apiClient, err := client.New(
		client.WithHost(addr),
		client.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &Client{
		apiClient:  apiClient,
		authBase64: f.authBase64,
		liveRefStr: f.liveRefStr,
		dbRefStr:   f.dbRefStr,
	}, nil
}

type Client struct {
	apiClient  *client.Client
	authBase64 string
	liveRefStr string
	dbRefStr   string
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.apiClient.Ping(ctx, client.PingOptions{
		NegotiateAPIVersion: true,
	})
	if err != nil {
		return fmt.Errorf("ping: docker unavailable: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return nil
}

// DeployDB загружает образ базы данных, url которого указан в ClientFactoryConfig, создает и поднимает контейнер.
func (c *Client) DeployDB(ctx context.Context, containerName, dbUser, dbPassword string) (string, error) {
	const op = "deploy db"
	const dbName = "live"

	u, err := url.Parse(c.apiClient.DaemonHost())
	if err != nil {
		return "", fmt.Errorf("%s: parse daemon host: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", fmt.Errorf("%s: parse daemon host: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	config := &pgx.ConnConfig{
		Config: pgconn.Config{
			Host:     host,
			Port:     5432,
			User:     dbUser,
			Password: dbPassword,
			Database: dbName,
		},
	}
	dsn := config.ConnString()

	createResp, err := c.apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name:  containerName,
		Image: c.dbRefStr,
		Config: &container.Config{
			Env: []string{
				"POSTGRES_USER=" + dbUser,
				"POSTGRES_PASSWORD=" + dbPassword,
				"POSTGRES_DB=" + dbName,
			},
		},
		HostConfig: &container.HostConfig{
			PortBindings: network.PortMap{
				network.MustParsePort("5432"): []network.PortBinding{{
					HostIP:   netip.MustParseAddr("0.0.0.0"),
					HostPort: "5432",
				}},
			},
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyUnlessStopped},
		},
	})
	if err != nil {
		return "", fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	_, err = c.apiClient.ContainerStart(ctx, createResp.ID, client.ContainerStartOptions{})
	if err != nil {
		return "", fmt.Errorf("%s: container start: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return dsn, nil
}

// DeployWorker загружает образ live-воркера из приватного репозитория, создает и запускает контейнер.
func (c *Client) DeployWorker(ctx context.Context, workerName string, env []string) (containerID string, port int, err error) {
	const op = "deploy worker"

	pullResp, err := c.apiClient.ImagePull(ctx, c.liveRefStr, client.ImagePullOptions{
		RegistryAuth: c.authBase64,
	})
	if err != nil {
		return "", 0, fmt.Errorf("%s: image pull: %w", op, err)
	}
	if err := pullResp.Wait(ctx); err != nil {
		return "", 0, fmt.Errorf("%s: image pull wait: %w", op, err)
	}

	createResp, err := c.apiClient.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name:  workerName,
		Image: c.liveRefStr,
		Config: &container.Config{
			Env: env,
		},
		HostConfig: &container.HostConfig{
			PortBindings: network.PortMap{
				network.MustParsePort("50051"): []network.PortBinding{{
					HostIP:   netip.MustParseAddr("0.0.0.0"),
					HostPort: "0",
				}},
			},
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyUnlessStopped},
		},
	})
	if err != nil {
		return "", 0, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	if _, err = c.apiClient.ContainerStart(ctx, createResp.ID, client.ContainerStartOptions{}); err != nil {
		return "", 0, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	inspectResp, err := c.apiClient.ContainerInspect(ctx, createResp.ID, client.ContainerInspectOptions{})
	if err != nil {
		return "", 0, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	portStr := inspectResp.Container.NetworkSettings.Ports[network.MustParsePort("50051")][0].HostPort

	port, err = strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("%s: parse port: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return createResp.ID, port, nil
}

// StopWorker останавливает воркер.
func (c *Client) StopWorker(ctx context.Context, containerID string) error {
	_, err := c.apiClient.ContainerStop(ctx, containerID, client.ContainerStopOptions{})
	if err != nil {
		return fmt.Errorf("stop worker: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}

// StartWorker запускает остановленный воркер
func (c *Client) StartWorker(ctx context.Context, containerID string) (int, error) {
	const op = "start worker"
	_, err := c.apiClient.ContainerStart(ctx, containerID, client.ContainerStartOptions{})
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	inspectResp, err := c.apiClient.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	portStr := inspectResp.Container.NetworkSettings.Ports[network.MustParsePort("50051")][0].HostPort

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("%s: parse port: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return port, nil
}

// DeleteWorker останавливает и удаляет контейнер и том, с которым тот работает.
func (c *Client) DeleteWorker(ctx context.Context, containerID string) error {
	const op = "delete worker"
	err := c.StopWorker(ctx, containerID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	_, err = c.apiClient.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{
		RemoveVolumes: true,
	})
	if err != nil {
		return fmt.Errorf("%s: container remove: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return nil
}

// UpdateWorker обновляет образ, на котором работает воркер, останавливая и удаляя текущий контейнер и запуская новый.
func (c *Client) UpdateWorker(ctx context.Context, containerID string) (newContainerID string, port int, err error) {
	const op = "update worker"

	inspectResp, err := c.apiClient.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return "", 0, fmt.Errorf("%s: container inspect: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	if inspectResp.Container.Config == nil {
		return "", 0, fmt.Errorf("%s: container config is nil: %w", op, errorsx.ErrInternal)
	}

	if err = c.StopWorker(ctx, containerID); err != nil {
		return "", 0, fmt.Errorf("%s, %w", op, err)
	}

	if _, err = c.apiClient.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{}); err != nil {
		return "", 0, fmt.Errorf("%s: container remove: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	pullResp, err := c.apiClient.ImagePull(ctx, c.liveRefStr, client.ImagePullOptions{
		RegistryAuth: c.authBase64,
	})
	if err != nil {
		return "", 0, fmt.Errorf("%s: image pull: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	if err = pullResp.Wait(ctx); err != nil {
		return "", 0, fmt.Errorf("%s: image pull wait: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	newContainerID, port, err = c.DeployWorker(ctx, inspectResp.Container.Name, inspectResp.Container.Config.Env)
	if err != nil {
		return "", 0, fmt.Errorf("%s: %w", op, err)
	}

	return newContainerID, port, nil
}

// RestartWorker перезапускает воркер по id контейнера.
func (c *Client) RestartWorker(ctx context.Context, containerID string) error {
	_, err := c.apiClient.ContainerRestart(ctx, containerID, client.ContainerRestartOptions{})
	if err != nil {
		return fmt.Errorf("restart worker: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}

// StreamWorkerMetrics читает стрим ContainerStats, преобразуя в live.Metrics и отправляя их в канал.
func (c *Client) StreamWorkerMetrics(ctx context.Context, containerID string) (chan live.Metrics, error) {
	const op = "worker metrics"

	statsResp, err := c.apiClient.ContainerStats(ctx, containerID, client.ContainerStatsOptions{
		Stream:                true,
		IncludePreviousSample: true,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: container stats: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	metricsChan := make(chan live.Metrics, 10)

	go func() {
		defer statsResp.Body.Close()
		defer close(metricsChan)

		decoder := json.NewDecoder(statsResp.Body)

		for {
			if ctx.Err() != nil {
				return
			}

			var stats container.StatsResponse
			if err := decoder.Decode(&stats); err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				return
			}

			metrics := c.calculateMetrics(stats)

			select {
			case <-ctx.Done():
				return
			case metricsChan <- metrics:
			}
		}
	}()

	return metricsChan, nil
}

// calculateMetrics рассчитывает метрики контейнера
func (c *Client) calculateMetrics(stats container.StatsResponse) live.Metrics {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage) - float64(stats.PreCPUStats.SystemUsage)

	cpuPercent := 0.0
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		onlineCPUs := float64(stats.CPUStats.OnlineCPUs)
		if onlineCPUs == 0 {
			onlineCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
		}
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}

	// Расчет памяти (типичная формула для Docker)
	memUsage := stats.MemoryStats.Usage
	if inactiveFile, ok := stats.MemoryStats.Stats["total_inactive_file"]; ok {
		memUsage -= inactiveFile
	}

	return live.Metrics{
		ContainerID: stats.ID,
		CPUPercent:  cpuPercent,
		MemoryBytes: memUsage,
	}
}
