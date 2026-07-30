package live

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/detect/detector"
	"github.com/pulsoats/core/detect/filter"
	"github.com/pulsoats/core/envvars"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	"github.com/pulsoats/main/internal/domain/live"
)

func workerContainerName(accountName string) string {
	name := strings.ToLower(accountName)
	name = strings.ReplaceAll(name, " ", "-")
	// убрать всё кроме букв, цифр, дефиса
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	name = reg.ReplaceAllString(name, "")
	return fmt.Sprintf("worker-%s", name)
}

func (a *Application) CreateWorker(ctx context.Context, accountID uuid.UUID) (live.Worker, error) {
	const op = "create worker"

	account, err := a.cfg.AccountRepo.AccountByIDWithCredentials(ctx, accountID)
	if err != nil {
		return live.Worker{}, fmt.Errorf("%s: %w", op, err)
	}
	if account.Credentials == nil {
		return live.Worker{}, fmt.Errorf("%s: account credentials is nil: %w", op, errorsx.ErrInternal)
	}

	node, err := a.cfg.NodeRepo.LeastLoadedNodeByExchange(ctx, account.Exchange)
	if err != nil {
		return live.Worker{}, fmt.Errorf("%s: no available node: %w", op, err)
	}

	if node.Status != live.NodeStatusActive {
		return live.Worker{}, fmt.Errorf("%s: node not active: %w", op, errorsx.ErrConflict)
	}

	ip := net.ParseIP(node.Host)
	if ip == nil {
		return live.Worker{}, fmt.Errorf("%s: invalid host: %w", op, errorsx.ErrInternal)
	}

	cert, key, err := a.cfg.CertGenerator.GenerateServerCert(ip)
	if err != nil {
		return live.Worker{}, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	env := []string{
		envvars.LivePostgresDSN + "=" + node.DSN,
		envvars.LiveExchangeCode + "=" + node.Exchange,
		envvars.LiveExchangeAPIKey + "=" + account.Credentials.APIKey,
		envvars.LiveExchangeAPISecret + "=" + account.Credentials.APISecret,
		envvars.LiveExchangePassphrase + "=" + account.Credentials.Passphrase,
		envvars.LiveGRPCCACert + "=" + a.cfg.GRPCCACert,
		envvars.LiveGRPCTLSCert + "=" + string(cert),
		envvars.LiveGRPCTLSKey + "=" + string(key),
	}

	addr := "tcp://" + net.JoinHostPort(node.Host, strconv.Itoa(node.DockerPort))
	dockerClient, err := a.cfg.DockerClientFactory.NewClient(addr)
	if err != nil {
		return live.Worker{}, fmt.Errorf("%s: %w", op, err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return live.Worker{}, fmt.Errorf("%s: generate id: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	worker := live.Worker{
		ID:                id,
		NodeID:            node.ID,
		Host:              node.Host,
		ExchangeAccountID: account.ID,
		Status:            live.WorkerStatusDeploying,
	}

	workerName := workerContainerName(account.Name)

	if err = a.cfg.WorkerRepo.CreateWorker(ctx, &worker); err != nil {
		return live.Worker{}, err
	}

	go func() {
		deployCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		containerID, port, err := dockerClient.DeployWorker(deployCtx, workerName, env)
		if err != nil {
			a.failWorker(worker.ID, op, err)
			return
		}

		grpcAddr := net.JoinHostPort(worker.Host, strconv.Itoa(port))
		client, err := a.cfg.WorkerClientFactory.NewClient(grpcAddr)
		if err != nil {
			a.failWorker(worker.ID, op, err)
			return
		}

		if err = waitHealthy(deployCtx, client, 30*time.Second, 2*time.Second); err != nil {
			a.failWorker(worker.ID, op, err)
			return
		}

		if err = a.cfg.WorkerRepo.UpdateWorkerDeploymentByID(deployCtx, worker.ID, containerID, port); err != nil {
			a.failWorker(worker.ID, op, err)
			return
		}

		a.clientsMu.Lock()
		a.workerClients[account.ID] = client
		a.clientsMu.Unlock()

		if err = a.cfg.WorkerRepo.UpdateWorkerStatusByID(deployCtx, worker.ID, live.WorkerStatusRunning, nil); err != nil {
			a.failWorker(worker.ID, op, err)
		}
	}()

	return worker, nil
}

func (a *Application) StopWorker(ctx context.Context, accountID uuid.UUID, callerID uuid.UUID) error {
	const op = "stop worker"

	worker, err := a.cfg.WorkerRepo.WorkerByAccountID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	a.clientsMu.RLock()
	grpcClient, grpcOk := a.workerClients[worker.ExchangeAccountID]
	dockerClient, dockerOk := a.nodeClients[worker.NodeID]
	a.clientsMu.RUnlock()

	if !grpcOk || !dockerOk {
		a.cfg.Logger.Error("client state inconsistency",
			"op", op,
			"grpc_client", grpcOk,
			"docker_client", dockerOk,
			"worker_id", worker.ID,
			"node_id", worker.NodeID,
		)
		return fmt.Errorf("%s: %w", op, errorsx.ErrInternal)
	}

	if err := grpcClient.StopAllRuns(ctx, callerID); err != nil {
		a.cfg.Logger.Warn(op, "error", err)
	}

	if err := dockerClient.StopWorker(ctx, worker.ContainerID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	a.clientsMu.Lock()
	delete(a.workerClients, worker.ExchangeAccountID)
	a.clientsMu.Unlock()

	if err := a.cfg.WorkerRepo.UpdateWorkerStatusByID(ctx, worker.ID, live.WorkerStatusStopped, nil); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (a *Application) StartWorker(ctx context.Context, accountID uuid.UUID) (live.Worker, error) {
	const op = "start worker"

	worker, err := a.cfg.WorkerRepo.WorkerByAccountID(ctx, accountID)
	if err != nil {
		return live.Worker{}, fmt.Errorf("%s: %w", op, err)
	}

	if worker.Status == live.WorkerStatusRunning {
		return live.Worker{}, fmt.Errorf("%s: worker already running: %w", op, errorsx.ErrConflict)
	}

	a.clientsMu.RLock()
	dockerClient, ok := a.nodeClients[worker.NodeID]
	a.clientsMu.RUnlock()
	if !ok {
		return live.Worker{}, fmt.Errorf("%s: node client not initialized: %w", op, errorsx.ErrInternal)
	}

	if err = a.cfg.WorkerRepo.UpdateWorkerStatusByID(ctx, worker.ID, live.WorkerStatusDeploying, nil); err != nil {
		return live.Worker{}, fmt.Errorf("%s: %w", op, err)
	}

	worker.Status = live.WorkerStatusDeploying

	go func() {
		startCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		port, err := dockerClient.StartWorker(startCtx, worker.ContainerID)
		if err != nil {
			a.failWorker(worker.ID, op, err)
			return
		}

		grpcAddr := net.JoinHostPort(worker.Host, strconv.Itoa(port))
		client, err := a.cfg.WorkerClientFactory.NewClient(grpcAddr)
		if err != nil {
			a.failWorker(worker.ID, op, err)
			return
		}

		if err = client.HealthCheck(startCtx); err != nil {
			a.failWorker(worker.ID, op, err)
			return
		}

		if err = a.cfg.WorkerRepo.UpdateWorkerDeploymentByID(startCtx, worker.ID, worker.ContainerID, port); err != nil {
			a.failWorker(worker.ID, op, err)
			return
		}

		a.clientsMu.Lock()
		a.workerClients[accountID] = client
		a.clientsMu.Unlock()

		if err = a.cfg.WorkerRepo.UpdateWorkerStatusByID(startCtx, worker.ID, live.WorkerStatusRunning, nil); err != nil {
			a.failWorker(worker.ID, op, err)
		}
	}()

	return worker, nil
}

func (a *Application) UpdateWorker(ctx context.Context, accountID uuid.UUID) (live.Worker, error) {
	const op = "update worker"

	worker, err := a.cfg.WorkerRepo.WorkerByAccountID(ctx, accountID)
	if err != nil {
		return live.Worker{}, fmt.Errorf("%s: %w", op, err)
	}

	if worker.Status == live.WorkerStatusRunning {
		return live.Worker{}, fmt.Errorf("%s: worker already running: %w", op, errorsx.ErrConflict)
	}

	a.clientsMu.RLock()
	dockerClient, ok := a.nodeClients[worker.NodeID]
	a.clientsMu.RUnlock()
	if !ok {
		return live.Worker{}, fmt.Errorf("%s: node client not initialized: %w", op, errorsx.ErrInternal)
	}

	if err = a.cfg.WorkerRepo.UpdateWorkerStatusByID(ctx, worker.ID, live.WorkerStatusDeploying, nil); err != nil {
		return live.Worker{}, fmt.Errorf("%s: %w", op, err)
	}

	worker.Status = live.WorkerStatusDeploying

	go func() {
		updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		newContainerID, port, err := dockerClient.UpdateWorker(updateCtx, worker.ContainerID)
		if err != nil {
			a.failWorker(worker.ID, op, err)
			return
		}

		grpcAddr := net.JoinHostPort(worker.Host, strconv.Itoa(port))
		client, err := a.cfg.WorkerClientFactory.NewClient(grpcAddr)
		if err != nil {
			a.failWorker(worker.ID, op, err)
			return
		}

		if err = waitHealthy(updateCtx, client, 30*time.Second, 2*time.Second); err != nil {
			a.failWorker(worker.ID, op, err)
			return
		}

		if err = a.cfg.WorkerRepo.UpdateWorkerDeploymentByID(updateCtx, worker.ID, newContainerID, port); err != nil {
			a.failWorker(worker.ID, op, err)
			return
		}

		a.clientsMu.Lock()
		a.workerClients[accountID] = client
		a.clientsMu.Unlock()

		if err = a.cfg.WorkerRepo.UpdateWorkerStatusByID(updateCtx, worker.ID, live.WorkerStatusRunning, nil); err != nil {
			a.failWorker(worker.ID, op, err)
		}
	}()

	return worker, nil
}

func (a *Application) WorkerByID(ctx context.Context, workerID uuid.UUID) (live.Worker, error) {
	return a.cfg.WorkerRepo.WorkerByID(ctx, workerID)
}

func (a *Application) WorkerByExchangeAccountID(ctx context.Context, accountID uuid.UUID) (live.Worker, error) {
	return a.cfg.WorkerRepo.WorkerByAccountID(ctx, accountID)
}

func (a *Application) Workers(ctx context.Context, f WorkersFilter) ([]live.Worker, error) {
	switch {
	case f.NodeID != nil:
		return a.cfg.WorkerRepo.WorkersByNodeID(ctx, *f.NodeID)
	case f.Exchange != nil:
		return a.cfg.WorkerRepo.WorkersByExchange(ctx, *f.Exchange)
	default:
		return a.cfg.WorkerRepo.Workers(ctx)
	}
}

type WorkersFilter struct {
	Exchange *string
	NodeID   *uuid.UUID
}

// WorkerMetrics собирает состояние воркера из двух независимых источников.
// Ошибка — только если воркера нет в БД. Недоступность любого из источников
// даёт nil в соответствующем поле, а не общий отказ: контейнер может быть жив,
// когда воркер завис, и наоборот.
func (a *Application) WorkerMetrics(ctx context.Context, exchangeAccountID uuid.UUID) (live.WorkerMetrics, error) {
	const op = "worker health"

	worker, err := a.cfg.WorkerRepo.WorkerByAccountID(ctx, exchangeAccountID)
	if err != nil {
		return live.WorkerMetrics{}, fmt.Errorf("%s: %w", op, err)
	}

	a.clientsMu.RLock()
	dockerClient, dockerOK := a.nodeClients[worker.NodeID]
	workerClient, workerOK := a.workerClients[exchangeAccountID]
	a.clientsMu.RUnlock()

	var (
		wg             sync.WaitGroup
		containerStats *live.ResourceUsage
		workerStats    *live.WorkerStats
	)

	if dockerOK {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cstats, err := dockerClient.ContainerStats(ctx, worker.ContainerID)
			if err != nil {
				a.cfg.Logger.Warn("container stats unavailable", "op", op, "worker_id", worker.ID, "error", err)
				return
			}
			containerStats = &cstats
		}()
	}

	if workerOK {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := workerClient.WorkerStats(ctx)
			if err != nil {
				a.cfg.Logger.Warn("worker stats unavailable", "op", op, "worker_id", worker.ID, "error", err)
				return
			}
			workerStats = &s
		}()
	}

	wg.Wait()

	return live.WorkerMetrics{
		WorkerID:      worker.ID,
		Status:        worker.Status,
		WorkerStats:   workerStats,
		ResourceUsage: containerStats,
		At:            time.Now(),
	}, nil
}

func (a *Application) AvailableDetectors(ctx context.Context, accountID uuid.UUID) ([]detector.Meta, error) {
	const op = "available detectors"

	a.clientsMu.RLock()
	client, ok := a.workerClients[accountID]
	if !ok {
		a.clientsMu.RUnlock()
		return nil, fmt.Errorf("%s: worker: %w", op, errorsx.ErrNotFound)
	}
	a.clientsMu.RUnlock()

	result, err := client.AvailableDetectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: client: %w", op, err)
	}

	return result, nil
}

func (a *Application) AvailableFilters(ctx context.Context, accountID uuid.UUID) ([]filter.Meta, error) {
	const op = "available filters"

	a.clientsMu.RLock()
	client, ok := a.workerClients[accountID]
	if !ok {
		a.clientsMu.RUnlock()
		return nil, fmt.Errorf("%s: worker: %w", op, errorsx.ErrNotFound)
	}
	a.clientsMu.RUnlock()

	result, err := client.AvailableFilters(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: client: %w", op, err)
	}

	return result, nil
}

func (a *Application) AvailableExchanges(ctx context.Context, accountID uuid.UUID) ([]exchange.Meta, error) {
	const op = "available exchanges"

	a.clientsMu.RLock()
	client, ok := a.workerClients[accountID]
	if !ok {
		a.clientsMu.RUnlock()
		return nil, fmt.Errorf("%s: worker: %w", op, errorsx.ErrNotFound)
	}
	a.clientsMu.RUnlock()

	result, err := client.AvailableExchanges(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: client: %w", op, err)
	}

	return result, nil
}

func (a *Application) NewRun(ctx context.Context, accountID uuid.UUID, callerID uuid.UUID, req live.NewRunRequest) (live.Run, error) {
	const op = "new run"

	a.clientsMu.RLock()
	client, ok := a.workerClients[accountID]
	if !ok {
		a.clientsMu.RUnlock()
		return live.Run{}, fmt.Errorf("%s: worker: %w", op, errorsx.ErrNotFound)
	}
	a.clientsMu.RUnlock()

	resp, err := client.NewRun(ctx, callerID, req)
	if err != nil {
		return live.Run{}, fmt.Errorf("%s: client: %w", op, err)
	}

	_ = a.cfg.MarketRepo.UpsertSymbols(ctx, req.MarketSpec.Exchange, req.MarketSpec.Category, []string{req.MarketSpec.Symbol})

	return resp, nil
}

func (a *Application) RestartRun(ctx context.Context, accountID uuid.UUID, runID uuid.UUID, callerID uuid.UUID) (live.Run, error) {
	const op = "restart run"

	a.clientsMu.RLock()
	client, ok := a.workerClients[accountID]
	if !ok {
		a.clientsMu.RUnlock()
		return live.Run{}, fmt.Errorf("%s: worker: %w", op, errorsx.ErrNotFound)
	}
	a.clientsMu.RUnlock()

	resp, err := client.RestartRun(ctx, runID, callerID)
	if err != nil {
		return live.Run{}, fmt.Errorf("%s: client: %w", op, err)
	}

	return resp, nil
}

func (a *Application) StopRun(ctx context.Context, accountID uuid.UUID, runID uuid.UUID, callerID uuid.UUID) error {
	const op = "stop run"

	a.clientsMu.RLock()
	client, ok := a.workerClients[accountID]
	if !ok {
		a.clientsMu.RUnlock()
		return fmt.Errorf("%s: worker: %w", op, errorsx.ErrNotFound)
	}
	a.clientsMu.RUnlock()

	if err := client.StopRun(ctx, runID, callerID); err != nil {
		return fmt.Errorf("%s: client: %w", op, err)
	}

	return nil
}

func (a *Application) StopAllRuns(ctx context.Context, accountID uuid.UUID, callerID uuid.UUID) error {
	const op = "stop all runs"

	a.clientsMu.RLock()
	client, ok := a.workerClients[accountID]
	if !ok {
		a.clientsMu.RUnlock()
		return fmt.Errorf("%s: worker: %w", op, errorsx.ErrNotFound)
	}
	a.clientsMu.RUnlock()

	if err := client.StopAllRuns(ctx, callerID); err != nil {
		return fmt.Errorf("%s: client: %w", op, err)
	}

	return nil
}

func (a *Application) Run(ctx context.Context, accountID uuid.UUID, runID uuid.UUID) (live.Run, error) {
	const op = "get run"

	a.clientsMu.RLock()
	client, ok := a.workerClients[accountID]
	if !ok {
		a.clientsMu.RUnlock()
		return live.Run{}, fmt.Errorf("%s: worker: %w", op, errorsx.ErrNotFound)
	}
	a.clientsMu.RUnlock()

	resp, err := client.RunByID(ctx, runID)
	if err != nil {
		return live.Run{}, fmt.Errorf("%s: client: %w", op, err)
	}

	return resp, nil
}

func (a *Application) RunsPaged(ctx context.Context, accountID uuid.UUID, req live.RunsPagedRequest) (live.RunsPagedResponse, error) {
	const op = "runs paged"

	a.clientsMu.RLock()
	client, ok := a.workerClients[accountID]
	if !ok {
		a.clientsMu.RUnlock()
		return live.RunsPagedResponse{}, fmt.Errorf("%s: worker: %w", op, errorsx.ErrNotFound)
	}
	a.clientsMu.RUnlock()

	resp, err := client.RunsPaged(ctx, req)
	if err != nil {
		return live.RunsPagedResponse{}, fmt.Errorf("%s: client: %w", op, err)
	}

	return resp, nil
}

func (a *Application) SignalsPaged(ctx context.Context, accountID uuid.UUID, req live.SignalsPagedRequest) (live.SignalsPagedResponse, error) {
	const op = "signals paged"

	a.clientsMu.RLock()
	client, ok := a.workerClients[accountID]
	if !ok {
		a.clientsMu.RUnlock()
		return live.SignalsPagedResponse{}, fmt.Errorf("%s: worker: %w", op, errorsx.ErrNotFound)
	}
	a.clientsMu.RUnlock()

	resp, err := client.SignalsPaged(ctx, req)
	if err != nil {
		return live.SignalsPagedResponse{}, fmt.Errorf("%s: client: %w", op, err)
	}

	return resp, nil
}

func (a *Application) StreamEvents(ctx context.Context, accountID uuid.UUID) (<-chan live.Event, error) {
	const op = "stream events"

	a.clientsMu.RLock()
	client, ok := a.workerClients[accountID]
	if !ok {
		a.clientsMu.RUnlock()
		return nil, fmt.Errorf("%s: worker: %w", op, errorsx.ErrNotFound)
	}
	a.clientsMu.RUnlock()

	ch, err := client.StreamEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: client: %w", op, err)
	}

	return ch, nil
}

func (a *Application) failWorker(workerID uuid.UUID, op string, deployErr error) {
	failCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var deployErrStr *string
	if deployErr != nil {
		deployErrStr = new(deployErr.Error())
	}

	if err := a.cfg.WorkerRepo.UpdateWorkerStatusByID(failCtx, workerID, live.WorkerStatusFailed, deployErrStr); err != nil {
		a.cfg.Logger.Error("failed to update worker status",
			"op", op,
			"repo_error", err.Error(),
			"deploy_error", deployErr,
			"worker_id", workerID,
		)
	}
}

type healthChecker interface {
	HealthCheck(ctx context.Context) error
}

func waitHealthy(ctx context.Context, c healthChecker, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if lastErr = c.HealthCheck(ctx); lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return fmt.Errorf("worker not healthy after %s: %w", timeout, lastErr)
}
