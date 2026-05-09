package live

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	coremarket "github.com/pulsoats/core/market"
	coresystem "github.com/pulsoats/core/system"

	"github.com/pulsoats/main/internal/domain/live"
	domainmarket "github.com/pulsoats/main/internal/domain/market"
	domainsystem "github.com/pulsoats/main/internal/domain/system"
)

type RunClient interface {
	NewRun(ctx context.Context, market coremarket.Spec, interval string, detector detect.DetectorConfig, callerID uuid.UUID) (live.Run, error)
	RestartRun(ctx context.Context, runID uuid.UUID, callerID uuid.UUID) (live.Run, error)
	StopRun(ctx context.Context, runID uuid.UUID, callerID uuid.UUID) error
	StopAll(ctx context.Context, callerID uuid.UUID) error
	GetRun(ctx context.Context, runID uuid.UUID) (live.Run, error)
	ListRunsPaged(ctx context.Context, req live.ListRunsRequest) (live.ListRunsResponse, error)
	ListSignalsPaged(ctx context.Context, req live.ListSignalsPagedRequest) (live.ListSignalsPagedResponse, error)
	StreamEvents(ctx context.Context) (*live.Stream, error)
}

type CatalogClient interface {
	ListAvailableDetectors(ctx context.Context) ([]detect.DetectorMeta, error)
	ListAvailableExchanges(ctx context.Context) ([]exchange.Meta, error)
}

type MonitorClient interface {
	Info(ctx context.Context) (coresystem.ServiceInfo, error)
	Metrics(ctx context.Context) (coresystem.ServiceMetrics, error)
}

type Client interface {
	RunClient
	CatalogClient
	MonitorClient
}

type PoolEntry struct {
	Info   coresystem.ServiceInfo
	Client Client
}

type Pool interface {
	Register(ctx context.Context, addr string) (coresystem.ServiceInfo, error)
	Get(id uuid.UUID) (Client, bool)
	GetByExchangeAccount(exchange, account string) (uuid.UUID, bool)
	Remove(id uuid.UUID)
	List() []PoolEntry
}

type Application struct {
	pool       Pool
	repo       domainsystem.Repository
	marketRepo domainmarket.Repository
}

func NewApplication(pool Pool, repo domainsystem.Repository, marketRepo domainmarket.Repository) *Application {
	return &Application{pool: pool, repo: repo, marketRepo: marketRepo}
}

func (a *Application) Register(ctx context.Context, addr string) (coresystem.ServiceInfo, error) {
	info, err := a.pool.Register(ctx, addr)
	if err != nil {
		return coresystem.ServiceInfo{}, fmt.Errorf("register: %w", err)
	}

	svc := &domainsystem.Service{
		ServiceInfo: info,
		Addr:        addr,
	}
	if err := a.repo.CreateService(ctx, svc); err != nil {
		return coresystem.ServiceInfo{}, fmt.Errorf("register: persist: %w", err)
	}

	return info, nil
}

// LoadFromDB восстанавливает пул из БД при старте.
// Сервисы, к которым не удалось подключиться, пропускаются — ошибки возвращаются списком.
func (a *Application) LoadFromDB(ctx context.Context) []error {
	services, err := a.repo.ListServices(ctx)
	if err != nil {
		return []error{fmt.Errorf("load from db: list: %w", err)}
	}

	var errs []error
	for _, svc := range services {
		if _, err := a.pool.Register(ctx, svc.Addr); err != nil {
			errs = append(errs, fmt.Errorf("load from db: %s (%s): %w", svc.Name, svc.Addr, err))
		}
	}
	return errs
}

func (a *Application) ServiceIDByExchangeAccount(exchange, account string) (uuid.UUID, bool) {
	return a.pool.GetByExchangeAccount(exchange, account)
}

func (a *Application) List() []PoolEntry {
	return a.pool.List()
}

func (a *Application) ListServices(ctx context.Context) ([]domainsystem.Service, error) {
	services, err := a.repo.ListServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	return services, nil
}

func (a *Application) Remove(ctx context.Context, exchange, account string) error {
	id, ok := a.pool.GetByExchangeAccount(exchange, account)
	if !ok {
		return fmt.Errorf("remove: service %s/%s: %w", exchange, account, errorsx.ErrNotFound)
	}
	if err := a.repo.DeleteService(ctx, exchange, account); err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	a.pool.Remove(id)
	return nil
}

func (a *Application) NewRun(ctx context.Context, serviceID uuid.UUID, mkt coremarket.Spec, interval string, detector detect.DetectorConfig, callerID uuid.UUID) (live.Run, error) {
	client, ok := a.pool.Get(serviceID)
	if !ok {
		return live.Run{}, fmt.Errorf("new run: service %s: %w", serviceID, errorsx.ErrNotFound)
	}

	resp, err := client.NewRun(ctx, mkt, interval, detector, callerID)
	if err != nil {
		return live.Run{}, fmt.Errorf("new run: client: %w", err)
	}

	_ = a.marketRepo.UpsertSymbols(ctx, mkt.Exchange, mkt.Category, []string{mkt.Symbol})

	return resp, nil
}

func (a *Application) RestartRun(ctx context.Context, serviceID uuid.UUID, runID uuid.UUID, callerID uuid.UUID) (live.Run, error) {
	client, ok := a.pool.Get(serviceID)
	if !ok {
		return live.Run{}, fmt.Errorf("restart run: service %s: %w", serviceID, errorsx.ErrNotFound)
	}

	resp, err := client.RestartRun(ctx, runID, callerID)
	if err != nil {
		return live.Run{}, fmt.Errorf("restart run: client: %w", err)
	}

	return resp, nil
}

func (a *Application) StopRun(ctx context.Context, serviceID uuid.UUID, runID uuid.UUID, callerID uuid.UUID) error {
	client, ok := a.pool.Get(serviceID)
	if !ok {
		return fmt.Errorf("stop run: service %s: %w", serviceID, errorsx.ErrNotFound)
	}

	if err := client.StopRun(ctx, runID, callerID); err != nil {
		return fmt.Errorf("stop run: client: %w", err)
	}

	return nil
}

func (a *Application) StopAll(ctx context.Context, serviceID uuid.UUID, callerID uuid.UUID) error {
	client, ok := a.pool.Get(serviceID)
	if !ok {
		return fmt.Errorf("stop all: service %s: %w", serviceID, errorsx.ErrNotFound)
	}

	if err := client.StopAll(ctx, callerID); err != nil {
		return fmt.Errorf("stop all: client: %w", err)
	}

	return nil
}

func (a *Application) GetRun(ctx context.Context, serviceID uuid.UUID, runID uuid.UUID) (live.Run, error) {
	client, ok := a.pool.Get(serviceID)
	if !ok {
		return live.Run{}, fmt.Errorf("get run: service %s: %w", serviceID, errorsx.ErrNotFound)
	}

	resp, err := client.GetRun(ctx, runID)
	if err != nil {
		return live.Run{}, fmt.Errorf("get run: client: %w", err)
	}

	return resp, nil
}

func (a *Application) ListRunsPaged(ctx context.Context, serviceID uuid.UUID, req live.ListRunsRequest) (live.ListRunsResponse, error) {
	client, ok := a.pool.Get(serviceID)
	if !ok {
		return live.ListRunsResponse{}, fmt.Errorf("list runs: service %s: %w", serviceID, errorsx.ErrNotFound)
	}

	resp, err := client.ListRunsPaged(ctx, req)
	if err != nil {
		return live.ListRunsResponse{}, fmt.Errorf("list runs: client: %w", err)
	}

	return resp, nil
}

func (a *Application) ListSignalsPaged(ctx context.Context, serviceID uuid.UUID, req live.ListSignalsPagedRequest) (live.ListSignalsPagedResponse, error) {
	client, ok := a.pool.Get(serviceID)
	if !ok {
		return live.ListSignalsPagedResponse{}, fmt.Errorf("list signals: service %s: %w", serviceID, errorsx.ErrNotFound)
	}

	resp, err := client.ListSignalsPaged(ctx, req)
	if err != nil {
		return live.ListSignalsPagedResponse{}, fmt.Errorf("list signals: client: %w", err)
	}

	return resp, nil
}

func (a *Application) StreamEvents(ctx context.Context, serviceID uuid.UUID) (*live.Stream, error) {
	client, ok := a.pool.Get(serviceID)
	if !ok {
		return nil, fmt.Errorf("stream events: service %s: %w", serviceID, errorsx.ErrNotFound)
	}

	resp, err := client.StreamEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("stream events: client: %w", err)
	}

	return resp, nil
}

func (a *Application) ListAvailableDetectors(ctx context.Context, serviceID uuid.UUID) ([]detect.DetectorMeta, error) {
	client, ok := a.pool.Get(serviceID)
	if !ok {
		return nil, fmt.Errorf("list available detectors: service %s: %w", serviceID, errorsx.ErrNotFound)
	}

	result, err := client.ListAvailableDetectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("list available detectors: client: %w", err)
	}

	return result, nil
}

func (a *Application) ListAvailableExchanges(ctx context.Context, serviceID uuid.UUID) ([]exchange.Meta, error) {
	client, ok := a.pool.Get(serviceID)
	if !ok {
		return nil, fmt.Errorf("list available exchanges: service %s: %w", serviceID, errorsx.ErrNotFound)
	}

	result, err := client.ListAvailableExchanges(ctx)
	if err != nil {
		return nil, fmt.Errorf("list available exchanges: client: %w", err)
	}

	return result, nil
}

func (a *Application) Info(ctx context.Context, serviceID uuid.UUID) (coresystem.ServiceInfo, error) {
	client, ok := a.pool.Get(serviceID)
	if !ok {
		return coresystem.ServiceInfo{}, fmt.Errorf("info: service %s: %w", serviceID, errorsx.ErrNotFound)
	}

	info, err := client.Info(ctx)
	if err != nil {
		return coresystem.ServiceInfo{}, fmt.Errorf("info: client: %w", err)
	}

	return info, nil
}

func (a *Application) Metrics(ctx context.Context, serviceID uuid.UUID) (coresystem.ServiceMetrics, error) {
	client, ok := a.pool.Get(serviceID)
	if !ok {
		return coresystem.ServiceMetrics{}, fmt.Errorf("metrics: service %s: %w", serviceID, errorsx.ErrNotFound)
	}

	metrics, err := client.Metrics(ctx)
	if err != nil {
		return coresystem.ServiceMetrics{}, fmt.Errorf("metrics: client: %w", err)
	}

	return metrics, nil
}
