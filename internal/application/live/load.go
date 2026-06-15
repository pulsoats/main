package live

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/pulsoats/main/internal/domain/live"
)

// LoadFromDB восстанавливает nodeClients и workerClients из БД при старте.
// Возвращает список ошибок — не фатальные, каждый сбой логируется, остальные клиенты восстанавливаются.
func (a *Application) LoadFromDB(ctx context.Context) []error {
	var errs []error

	nodes, err := a.nodeRepo.Nodes(ctx)
	if err != nil {
		return []error{fmt.Errorf("load from db: nodes: %w", err)}
	}

	a.clientsMu.Lock()
	defer a.clientsMu.Unlock()
	for _, node := range nodes {
		if node.Status != live.NodeStatusActive {
			continue
		}
		addr := "tcp://" + net.JoinHostPort(node.Host, strconv.Itoa(node.DockerPort))
		client, err := a.dockerFactory.NewClient(addr)
		if err != nil {
			errs = append(errs, fmt.Errorf("load from db: node %s: docker client: %w", node.ID, err))
			continue
		}
		a.nodeClients[node.ID] = client
	}

	workers, err := a.workerRepo.Workers(ctx)
	if err != nil {
		return append(errs, fmt.Errorf("load from db: workers: %w", err))
	}

	for _, worker := range workers {
		if worker.Status != live.WorkerStatusRunning {
			continue
		}
		addr := net.JoinHostPort(worker.Host, strconv.Itoa(worker.GRPCPort))
		client, err := a.workerClientFactory.NewClient(addr)
		if err != nil {
			errs = append(errs, fmt.Errorf("load from db: worker %s: grpc client: %w", worker.ID, err))
			continue
		}
		a.workerClients[worker.ExchangeAccountID] = client
	}

	return errs
}
