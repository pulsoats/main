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
	RunsTotal    int32
	SignalsTotal int64
}
