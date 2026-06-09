package live

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type NodeStatus string

const (
	NodeStatusDeploying    NodeStatus = "deploying"
	NodeStatusActive       NodeStatus = "active"
	NodeStatusDisconnected NodeStatus = "disconnected"
	NodeStatusFailed       NodeStatus = "failed"
	NodeStatusDisabling    NodeStatus = "disabling"
	NodeStatusDisabled     NodeStatus = "disabled"
)

type Node struct {
	ID           uuid.UUID
	Exchange     string
	Host         string
	DockerPort   int
	Region       string
	MaxWorkers   int
	WorkersCount int
	DSN          string
	Status       NodeStatus
	LastError    *string
	CreatedAt    time.Time
}

type NodeRepository interface {
	CreateNode(ctx context.Context, node *Node) error
	NodeByID(ctx context.Context, id uuid.UUID) (Node, error)
	Nodes(ctx context.Context) ([]Node, error)
	NodesByExchange(ctx context.Context, exchange string) ([]Node, error)
	LeastLoadedNodeByExchange(ctx context.Context, exchange string) (Node, error)
	DeleteNodeByID(ctx context.Context, id uuid.UUID) error
	UpdateNodeStatusByID(ctx context.Context, nodeID uuid.UUID, status NodeStatus, nodeErr *string) error
	UpdateNodeDSNByID(ctx context.Context, nodeID uuid.UUID, dsn string) error
}

type AddNodeRequest struct {
	Exchange   string
	Host       string
	DockerPort int
	Region     string
	MaxWorkers int
	DBUser     string
	DBPassword string
}
