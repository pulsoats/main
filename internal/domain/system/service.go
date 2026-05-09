package system

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/system"
)

type Service struct {
	system.ServiceInfo
	Addr       string
	LastSeenAt time.Time
	CreatedAt  time.Time
}

type Repository interface {
	CreateService(ctx context.Context, s *Service) error
	ServiceByID(ctx context.Context, id uuid.UUID) (Service, error)
	ListServices(ctx context.Context) ([]Service, error)
	DeleteService(ctx context.Context, exchange, account string) error
}
