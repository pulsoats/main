package live

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/envvars"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	"github.com/pulsoats/core/market"
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

	account, err := a.accountRepo.AccountByIDWithCredentials(ctx, accountID)
	if err != nil {
		return live.Worker{}, fmt.Errorf("%s: %w", op, err)
	}
	if account.Credentials == nil {
		return live.Worker{}, fmt.Errorf("%s: account credentials is nil: %w", op, errorsx.ErrInternal)
	}

	node, err := a.nodeRepo.LeastLoadedNodeByExchange(ctx, account.Exchange)
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

	cert, key, err := a.certgen.GenerateServerCert(ip)
	if err != nil {
		return live.Worker{}, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	env := []string{
		envvars.LivePostgresDSN + "=" + node.DSN,
		envvars.LiveExchangeCode + "=" + node.Exchange,
		envvars.LiveExchangeAPIKey + "=" + account.Credentials.APIKey,
		envvars.LiveExchangeAPISecret + "=" + account.Credentials.APISecret,
		envvars.LiveExchangePassphrase + "=" + account.Credentials.Passphrase,
		envvars.LiveGRPCCACert + "=" + a.grpcCACert,
		envvars.LiveGRPCTLSCert + "=" + string(cert),
		envvars.LiveGRPCTLSKey + "=" + string(key),
	}

	addr := net.JoinHostPort(node.Host, strconv.Itoa(node.DockerPort))
	dockerClient, err := a.dockerFactory.NewClient(addr)
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

	if err = a.workerRepo.CreateWorker(ctx, &worker); err != nil {
		return live.Worker{}, err
	}

	fail := func(workerID uuid.UUID, deployErr error) {
		failCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var deployErrStr *string
		if deployErr != nil {
			s := deployErr.Error()
			deployErrStr = &s
		}

		if err := a.workerRepo.UpdateWorkerStatusByID(failCtx, workerID, live.WorkerStatusFailed, deployErrStr); err != nil {
			a.logger.Error("failed to update worker status",
				"op", op,
				"repo_error", err.Error(),
				"deploy_error", deployErr,
				"worker_id", workerID,
			)
		}
	}

	go func() {
		deployCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		containerID, port, err := dockerClient.DeployWorker(deployCtx, workerName, env)
		if err != nil {
			fail(worker.ID, err)
			return
		}

		grpcAddr := net.JoinHostPort(worker.Host, strconv.Itoa(port))
		client, err := a.workerClientFactory.NewClient(grpcAddr)
		if err != nil {
			fail(worker.ID, err)
			return
		}

		if err = client.HealthCheck(deployCtx); err != nil {
			fail(worker.ID, err)
			return
		}

		if err = a.workerRepo.UpdateWorkerDeploymentByID(deployCtx, worker.ID, containerID, port); err != nil {
			fail(worker.ID, err)
			return
		}

		a.clientsMu.Lock()
		a.workerClients[account.ID] = client
		a.clientsMu.Unlock()

		if err = a.workerRepo.UpdateWorkerStatusByID(deployCtx, worker.ID, live.WorkerStatusRunning, nil); err != nil {
			fail(worker.ID, err)
		}
	}()

	return worker, nil
}

func (a *Application) StopWorker(ctx context.Context, accountID uuid.UUID, callerID uuid.UUID) error {
	const op = "stop worker"

	worker, err := a.workerRepo.WorkerByAccountID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	a.clientsMu.RLock()
	grpcClient, grpcOk := a.workerClients[worker.ExchangeAccountID]
	dockerClient, dockerOk := a.nodeClients[worker.NodeID]
	a.clientsMu.RUnlock()

	if !grpcOk || !dockerOk {
		a.logger.Error("client state inconsistency",
			"op", op,
			"grpc_client", grpcOk,
			"docker_client", dockerOk,
			"worker_id", worker.ID,
			"node_id", worker.NodeID,
		)
		return fmt.Errorf("%s: %w", op, errorsx.ErrInternal)
	}

	if err := grpcClient.StopAllRuns(ctx, callerID); err != nil {
		a.logger.Warn(op, "error", err)
	}

	if err := dockerClient.StopWorker(ctx, worker.ContainerID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	a.clientsMu.Lock()
	delete(a.workerClients, worker.ExchangeAccountID)
	a.clientsMu.Unlock()

	if err := a.workerRepo.UpdateWorkerStatusByID(ctx, worker.ID, live.WorkerStatusStopped, nil); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (a *Application) StartWorker(ctx context.Context, accountID uuid.UUID) (live.Worker, error) {
	const op = "start worker"

	worker, err := a.workerRepo.WorkerByAccountID(ctx, accountID)
	if err != nil {
		return live.Worker{}, fmt.Errorf("%s: %w", op, err)
	}

	if worker.Status == live.WorkerStatusRunning {
		return live.Worker{}, fmt.Errorf("%s: worker already running: %w", op, errorsx.ErrConflict)
	}

	if worker.Status != live.WorkerStatusStopped {
		return live.Worker{}, fmt.Errorf("%s: worker clientsMust be stopped to start: %w", op, errorsx.ErrConflict)
	}

	a.clientsMu.RLock()
	dockerClient, ok := a.nodeClients[worker.NodeID]
	a.clientsMu.RUnlock()
	if !ok {
		return live.Worker{}, fmt.Errorf("%s: node client not initialized: %w", op, errorsx.ErrInternal)
	}

	if err = a.workerRepo.UpdateWorkerStatusByID(ctx, worker.ID, live.WorkerStatusDeploying, nil); err != nil {
		return live.Worker{}, fmt.Errorf("%s: %w", op, err)
	}

	worker.Status = live.WorkerStatusDeploying

	fail := func(deployErr error) {
		failCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var deployErrStr *string
		if deployErr != nil {
			s := deployErr.Error()
			deployErrStr = &s
		}

		if err := a.workerRepo.UpdateWorkerStatusByID(failCtx, worker.ID, live.WorkerStatusFailed, deployErrStr); err != nil {
			a.logger.Error("failed to update worker status",
				"op", op,
				"repo_error", err.Error(),
				"deploy_error", deployErr,
				"worker_id", worker.ID,
			)
		}
	}

	go func() {
		startCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		port, err := dockerClient.StartWorker(startCtx, worker.ContainerID)
		if err != nil {
			fail(err)
			return
		}

		grpcAddr := net.JoinHostPort(worker.Host, strconv.Itoa(port))
		client, err := a.workerClientFactory.NewClient(grpcAddr)
		if err != nil {
			fail(err)
			return
		}

		if err = client.HealthCheck(startCtx); err != nil {
			fail(err)
			return
		}

		if err = a.workerRepo.UpdateWorkerDeploymentByID(startCtx, worker.ID, worker.ContainerID, port); err != nil {
			fail(err)
			return
		}

		a.clientsMu.Lock()
		a.workerClients[accountID] = client
		a.clientsMu.Unlock()

		if err = a.workerRepo.UpdateWorkerStatusByID(startCtx, worker.ID, live.WorkerStatusRunning, nil); err != nil {
			fail(err)
		}
	}()

	return worker, nil
}

func (a *Application) WorkerByID(ctx context.Context, workerID uuid.UUID) (live.Worker, error) {
	return a.workerRepo.WorkerByID(ctx, workerID)
}

func (a *Application) WorkerByExchangeAccountID(ctx context.Context, accountID uuid.UUID) (live.Worker, error) {
	return a.workerRepo.WorkerByAccountID(ctx, accountID)
}

type WorkersFilter struct {
	Exchange *string
	NodeID   *uuid.UUID
}

func (a *Application) Workers(ctx context.Context, f WorkersFilter) ([]live.Worker, error) {
	switch {
	case f.NodeID != nil:
		return a.workerRepo.WorkersByNodeID(ctx, *f.NodeID)
	case f.Exchange != nil:
		return a.workerRepo.WorkersByExchange(ctx, *f.Exchange)
	default:
		return a.workerRepo.Workers(ctx)
	}
}

func (a *Application) SubscribeWorkerMetrics(ctx context.Context, exchangeAccountID uuid.UUID) (<-chan live.Metrics, error) {
	const op = "subscribe worker metrics"

	worker, err := a.workerRepo.WorkerByAccountID(ctx, exchangeAccountID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	a.clientsMu.RLock()
	dockerClient, ok := a.nodeClients[worker.NodeID]
	a.clientsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%s: node client not initialized: %w", op, errorsx.ErrInternal)
	}

	subID := uuid.New()
	ch := make(chan live.Metrics, 16)

	a.subsMu.Lock()
	firstSub := len(a.metricsSubs[exchangeAccountID]) == 0
	if a.metricsSubs[exchangeAccountID] == nil {
		a.metricsSubs[exchangeAccountID] = make(map[uuid.UUID]chan live.Metrics)
	}
	a.metricsSubs[exchangeAccountID][subID] = ch
	a.subsMu.Unlock()

	if firstSub {
		streamCtx, streamCancel := context.WithCancel(context.Background())
		metricsChan, err := dockerClient.StreamWorkerMetrics(streamCtx, worker.ContainerID)
		if err != nil {
			streamCancel()
			a.subsMu.Lock()
			delete(a.metricsSubs[exchangeAccountID], subID)
			delete(a.metricsSubs, exchangeAccountID)
			a.subsMu.Unlock()
			close(ch)
			return nil, fmt.Errorf("%s: stream: %w", op, err)
		}
		go a.broadcastWorkerMetrics(exchangeAccountID, metricsChan, streamCancel)
	}

	go func() {
		<-ctx.Done()
		a.subsMu.Lock()
		delete(a.metricsSubs[exchangeAccountID], subID)
		if len(a.metricsSubs[exchangeAccountID]) == 0 {
			delete(a.metricsSubs, exchangeAccountID)
		}
		a.subsMu.Unlock()
		close(ch)
	}()

	return ch, nil
}

func (a *Application) SubscribeWorkerStats(ctx context.Context, accountID uuid.UUID) (<-chan live.WorkerStats, error) {
	const op = "subscribe worker stats"

	a.clientsMu.RLock()
	client, ok := a.workerClients[accountID]
	a.clientsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%s: worker client not initialized: %w", op, errorsx.ErrInternal)
	}

	subID := uuid.New()
	ch := make(chan live.WorkerStats, 16)

	a.subsMu.Lock()
	firstSub := len(a.statsSubs[accountID]) == 0
	if a.statsSubs[accountID] == nil {
		a.statsSubs[accountID] = make(map[uuid.UUID]chan live.WorkerStats)
	}
	a.statsSubs[accountID][subID] = ch
	a.subsMu.Unlock()

	if firstSub {
		streamCtx, streamCancel := context.WithCancel(context.Background())
		statsChan, err := client.StreamWorkerStats(streamCtx)
		if err != nil {
			streamCancel()
			a.subsMu.Lock()
			delete(a.statsSubs[accountID], subID)
			delete(a.statsSubs, accountID)
			a.subsMu.Unlock()
			close(ch)
			return nil, fmt.Errorf("%s: stream: %w", op, err)
		}
		go a.broadcastWorkerStats(accountID, statsChan, streamCancel)
	}

	go func() {
		<-ctx.Done()
		a.subsMu.Lock()
		delete(a.statsSubs[accountID], subID)
		if len(a.statsSubs[accountID]) == 0 {
			delete(a.statsSubs, accountID)
		}
		a.subsMu.Unlock()
		close(ch)
	}()

	return ch, nil
}

func (a *Application) broadcastWorkerStats(accountID uuid.UUID, statsChan <-chan live.WorkerStats, cancel context.CancelFunc) {
	defer cancel()
	for stats := range statsChan {
		a.subsMu.RLock()
		subs := a.statsSubs[accountID]
		if len(subs) == 0 {
			a.subsMu.RUnlock()
			return
		}
		for _, ch := range subs {
			select {
			case ch <- stats:
			default:
			}
		}
		a.subsMu.RUnlock()
	}
}

func (a *Application) broadcastWorkerMetrics(accountID uuid.UUID, metricsChan <-chan live.Metrics, cancel context.CancelFunc) {
	defer cancel()
	for metrics := range metricsChan {
		a.subsMu.RLock()
		subs := a.metricsSubs[accountID]
		if len(subs) == 0 {
			a.subsMu.RUnlock()
			return
		}
		for _, ch := range subs {
			select {
			case ch <- metrics:
			default:
			}
		}
		a.subsMu.RUnlock()
	}
}

func (a *Application) AvailableDetectors(ctx context.Context, accountID uuid.UUID) ([]detect.DetectorMeta, error) {
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

func (a *Application) NewRun(ctx context.Context, accountID uuid.UUID, mkt market.Spec, interval string, detector detect.DetectorConfig, callerID uuid.UUID) (live.Run, error) {
	const op = "new run"

	a.clientsMu.RLock()
	client, ok := a.workerClients[accountID]
	if !ok {
		a.clientsMu.RUnlock()
		return live.Run{}, fmt.Errorf("%s: worker: %w", op, errorsx.ErrNotFound)
	}
	a.clientsMu.RUnlock()

	resp, err := client.NewRun(ctx, mkt, interval, detector, callerID)
	if err != nil {
		return live.Run{}, fmt.Errorf("%s: client: %w", op, err)
	}

	_ = a.marketRepo.UpsertSymbols(ctx, mkt.Exchange, mkt.Category, []string{mkt.Symbol})

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

	resp, err := client.GetRun(ctx, runID)
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
