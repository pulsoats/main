package live

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/domain/live"
)

func (a *Application) CreateNode(ctx context.Context, req live.AddNodeRequest) (live.Node, error) {
	const op = "add node"
	if ip := net.ParseIP(req.Host); ip == nil {
		return live.Node{}, fmt.Errorf("%s: ip: %w", op, errorsx.ErrInvalidArgument)
	}
	if req.DockerPort < 1 || req.DockerPort > 65535 {
		return live.Node{}, fmt.Errorf("%s: docker port: %w", op, errorsx.ErrInvalidArgument)
	}

	addr := "tcp://" + net.JoinHostPort(req.Host, strconv.Itoa(req.DockerPort))

	dockerClient, err := a.dockerFactory.NewClient(addr)
	if err != nil {
		return live.Node{}, fmt.Errorf("%s: docker: %w", op, err)
	}

	err = dockerClient.Ping(ctx)
	if err != nil {
		return live.Node{}, fmt.Errorf("%s: %w", op, err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return live.Node{}, fmt.Errorf("%s: generate id: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	node := live.Node{
		ID:           id,
		Exchange:     req.Exchange,
		DockerPort:   req.DockerPort,
		Region:       req.Region,
		MaxWorkers:   req.MaxWorkers,
		Status:       live.NodeStatusDeploying,
		WorkersCount: 0,
	}

	err = a.nodeRepo.CreateNode(ctx, &node)
	if err != nil {
		return live.Node{}, err
	}

	fail := func(nodeID uuid.UUID, deployErr error) {
		failCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var deployErrStr *string
		if deployErr != nil {
			s := deployErr.Error()
			deployErrStr = &s
		}

		if err := a.nodeRepo.UpdateNodeStatusByID(failCtx, nodeID, live.NodeStatusFailed, deployErrStr); err != nil {
			a.logger.Error("failed to update node status",
				"op", op,
				"repo_error", err.Error(),
				"deploy_error", deployErr,
				"node_id", nodeID,
			)
		}
	}

	go func() {
		deployCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		var dsn string
		if req.DSN != nil {
			dsn = *req.DSN
		} else {
			if req.DBUser == "" || req.DBPassword == "" {
				fail(node.ID, fmt.Errorf("dbUser and dbPassword are required when dsn is not provided"))
				return
			}
			containerName := fmt.Sprintf(dbContainerName, a.appName)
			var deployErr error
			dsn, deployErr = dockerClient.DeployDB(deployCtx, containerName, req.DBUser, req.DBPassword)
			if deployErr != nil {
				fail(node.ID, deployErr)
				return
			}
		}

		if err = a.nodeRepo.UpdateNodeDSNByID(deployCtx, node.ID, dsn); err != nil {
			fail(node.ID, err)
			return
		}

		a.clientsMu.Lock()
		a.nodeClients[node.ID] = dockerClient
		a.clientsMu.Unlock()

		if err = a.nodeRepo.UpdateNodeStatusByID(deployCtx, node.ID, live.NodeStatusActive, nil); err != nil {
			fail(node.ID, err)
		}
	}()

	return node, nil
}

func (a *Application) DisableNode(ctx context.Context, nodeID uuid.UUID, callerID uuid.UUID) error {
	const op = "disable node"

	// Check DB first so we can disable nodes whose client was not restored after restart.
	node, err := a.nodeRepo.NodeByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if node.Status == live.NodeStatusDisabled || node.Status == live.NodeStatusDisabling {
		return fmt.Errorf("%s: node: %w", op, errorsx.ErrConflict)
	}

	a.clientsMu.RLock()
	nodeClient, hasClient := a.nodeClients[nodeID]
	a.clientsMu.RUnlock()

	workers, err := a.workerRepo.WorkersByNodeID(ctx, nodeID, live.WorkerStatusRunning)
	if err != nil {
		return fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	if err = a.nodeRepo.UpdateNodeStatusByID(ctx, nodeID, live.NodeStatusDisabling, nil); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	go func() {
		disableCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		for _, w := range workers {
			a.clientsMu.RLock()
			client, ok := a.workerClients[w.ExchangeAccountID]
			a.clientsMu.RUnlock()

			if ok {
				if err := client.StopAllRuns(disableCtx, callerID); err != nil {
					a.logger.Error("failed to stop all runs", "op", op, "error", err, "worker_id", w.ID)
				}
			}

			if hasClient {
				if err := nodeClient.StopWorker(disableCtx, w.ContainerID); err != nil {
					s := err.Error()
					if saveErr := a.workerRepo.UpdateWorkerStatusByID(disableCtx, w.ID, live.WorkerStatusFailed, &s); saveErr != nil {
						a.logger.Error("failed to update worker status",
							"op", op,
							"repo_error", saveErr.Error(),
							"stop_error", err,
							"worker_id", w.ID,
						)
					}
					continue
				}
			} else {
				// Docker client not available — mark worker as stopped in DB best-effort.
				a.logger.Warn("docker client unavailable, marking worker stopped without container stop",
					"op", op, "worker_id", w.ID)
			}

			a.clientsMu.Lock()
			delete(a.workerClients, w.ExchangeAccountID)
			a.clientsMu.Unlock()

			if err := a.workerRepo.UpdateWorkerStatusByID(disableCtx, w.ID, live.WorkerStatusStopped, nil); err != nil {
				a.logger.Error("failed to update worker status",
					"op", op,
					"repo_error", err.Error(),
					"worker_id", w.ID,
				)
			}
		}

		a.clientsMu.Lock()
		delete(a.nodeClients, nodeID)
		a.clientsMu.Unlock()

		if err := a.nodeRepo.UpdateNodeStatusByID(disableCtx, nodeID, live.NodeStatusDisabled, nil); err != nil {
			a.logger.Error("failed to update node status",
				"op", op,
				"repo_error", err.Error(),
				"node_id", nodeID,
			)
		}
	}()

	return nil
}

func (a *Application) EnableNode(ctx context.Context, nodeID uuid.UUID) error {
	const op = "enable node"

	node, err := a.nodeRepo.NodeByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	workers, err := a.workerRepo.WorkersByNodeID(ctx, nodeID, live.WorkerStatusStopped, live.WorkerStatusRunning)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	addr := "tcp://" + net.JoinHostPort(node.Host, strconv.Itoa(node.DockerPort))
	dockerClient, err := a.dockerFactory.NewClient(addr)
	if err != nil {
		return fmt.Errorf("%s: docker: %w", op, err)
	}

	if err = dockerClient.Ping(ctx); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	a.clientsMu.Lock()
	a.nodeClients[nodeID] = dockerClient
	a.clientsMu.Unlock()

	go func() {
		fail := func(workerID uuid.UUID, workerErr error) {
			failCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			var workerErrStr *string
			if workerErr != nil {
				s := workerErr.Error()
				workerErrStr = &s
			}

			if err := a.workerRepo.UpdateWorkerStatusByID(failCtx, workerID, live.WorkerStatusFailed, workerErrStr); err != nil {
				a.logger.Error("failed to update worker status",
					"op", op,
					"repo_error", err.Error(),
					"deploy_error", workerErr,
					"worker_id", workerID,
				)
			}
		}

		for _, w := range workers {
			workerCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

			port, err := dockerClient.StartWorker(workerCtx, w.ContainerID)
			if err != nil {
				fail(w.ID, err)
				cancel()
				continue
			}

			grpcAddr := net.JoinHostPort(w.Host, strconv.Itoa(port))

			grpcClient, err := a.workerClientFactory.NewClient(grpcAddr)
			if err != nil {
				fail(w.ID, err)
				cancel()
				continue
			}

			if err = grpcClient.HealthCheck(workerCtx); err != nil {
				fail(w.ID, err)
				cancel()
				continue
			}

			a.clientsMu.Lock()
			a.workerClients[w.ExchangeAccountID] = grpcClient
			a.clientsMu.Unlock()

			if err = a.workerRepo.UpdateWorkerDeploymentByID(workerCtx, w.ID, w.ContainerID, port); err != nil {
				a.logger.Error("failed to update worker deployment",
					"op", op,
					"repo_error", err.Error(),
					"worker_id", w.ID,
				)
				cancel()
				continue
			}

			if err = a.workerRepo.UpdateWorkerStatusByID(workerCtx, w.ID, live.WorkerStatusRunning, nil); err != nil {
				a.logger.Error("failed to update worker status",
					"op", op,
					"repo_error", err.Error(),
					"worker_id", w.ID,
				)
			}
			cancel()
		}
	}()

	if err = a.nodeRepo.UpdateNodeStatusByID(ctx, nodeID, live.NodeStatusActive, nil); err != nil {
		a.logger.Error("failed to update node status",
			"op", op,
			"repo_error", err.Error(),
			"node_id", node.ID,
		)
	}
	return nil
}

func (a *Application) NodeByID(ctx context.Context, nodeID uuid.UUID) (live.Node, error) {
	return a.nodeRepo.NodeByID(ctx, nodeID)
}

type NodesFilter struct {
	Exchange *string
}

func (a *Application) Nodes(ctx context.Context, f NodesFilter) ([]live.Node, error) {
	if f.Exchange != nil {
		return a.nodeRepo.NodesByExchange(ctx, *f.Exchange)
	}
	return a.nodeRepo.Nodes(ctx)
}

func (a *Application) DeleteNode(ctx context.Context, nodeID uuid.UUID) error {
	const op = "delete node"

	node, err := a.nodeRepo.NodeByID(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if node.Status != live.NodeStatusDisabled {
		return fmt.Errorf("%s: only disabled node can be deleted: %w", op, errorsx.ErrConflict)
	}

	if err = a.nodeRepo.DeleteNodeByID(ctx, nodeID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
