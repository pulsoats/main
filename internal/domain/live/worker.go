package live

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type WorkerStatus string

const (
	WorkerStatusDeploying WorkerStatus = "deploying"
	WorkerStatusRunning   WorkerStatus = "running"
	WorkerStatusStopped   WorkerStatus = "stopped"
	WorkerStatusFailed    WorkerStatus = "failed"
)

type Worker struct {
	ID                uuid.UUID
	NodeID            uuid.UUID
	Host              string
	GRPCPort          int
	ContainerID       string
	ExchangeAccountID uuid.UUID
	Status            WorkerStatus
	LastError         *string
	CreatedAt         time.Time
}

type WorkersFilter struct {
}

type WorkerRepository interface {
	CreateWorker(ctx context.Context, worker *Worker) error
	WorkerByID(ctx context.Context, workerID uuid.UUID) (Worker, error)
	WorkerByAccountID(ctx context.Context, accountID uuid.UUID) (Worker, error)
	Workers(ctx context.Context) ([]Worker, error)
	WorkersByNodeID(ctx context.Context, nodeID uuid.UUID, statuses ...WorkerStatus) ([]Worker, error)
	WorkersByExchange(ctx context.Context, exchange string, statuses ...WorkerStatus) ([]Worker, error)
	DeleteWorkerByID(ctx context.Context, workerID uuid.UUID) error
	UpdateWorkerStatusByID(ctx context.Context, workerID uuid.UUID, status WorkerStatus, workerErr *string) error
	UpdateWorkerDeploymentByID(ctx context.Context, workerID uuid.UUID, containerID string, grpcPort int) error
}

type WorkerStats struct {
	ActiveRuns   int32
	SignalsTotal int32
}

type ResourceUsage struct {
	ContainerID string
	CPUPercent  float64
	MemoryBytes uint64
}

type WorkerMetrics struct {
	WorkerID      uuid.UUID
	Status        WorkerStatus
	WorkerStats   *WorkerStats
	ResourceUsage *ResourceUsage
	At            time.Time
}
