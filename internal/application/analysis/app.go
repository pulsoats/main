package analysis

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	coresystem "github.com/pulsoats/core/system"
	"github.com/pulsoats/main/internal/domain/analysis"
	"github.com/pulsoats/main/internal/domain/market"
)

type runClient interface {
	NewRun(ctx context.Context, callerID uuid.UUID, req analysis.NewRunRequest) (analysis.Run, error)
	RunByID(ctx context.Context, runID uuid.UUID) (analysis.Run, error)
	StreamRunArchive(ctx context.Context, runID uuid.UUID, dst io.Writer) error
	ShareRun(ctx context.Context, callerID, runID uuid.UUID) error
	DeleteRun(ctx context.Context, callerID, runID uuid.UUID) error
	ListRunsPaged(ctx context.Context, callerID uuid.UUID, req analysis.ListRunsPagedRequest) (analysis.ListRunsPagedResponse, error)
}

type catalogClient interface {
	ListAvailableDetectors(ctx context.Context) ([]detect.DetectorMeta, error)
	ListAvailableExchanges(ctx context.Context) ([]exchange.Meta, error)
}

type monitorClient interface {
	Info(ctx context.Context) (coresystem.ServiceInfo, error)
	Metrics(ctx context.Context) (coresystem.ServiceMetrics, error)
}

type Application struct {
	runClient     runClient
	catalogClient catalogClient
	marketRepo    market.Repository
	monitor       monitorClient
}

func NewApplication(runClient runClient, catalogClient catalogClient, marketRepo market.Repository, monitor monitorClient) (*Application, error) {
	if runClient == nil {
		return nil, fmt.Errorf("analysis app: run client is nil")
	}
	if catalogClient == nil {
		return nil, fmt.Errorf("analysis app: catalog client is nil")
	}
	return &Application{runClient: runClient, catalogClient: catalogClient, marketRepo: marketRepo, monitor: monitor}, nil
}

func (a *Application) NewRun(ctx context.Context, callerID uuid.UUID, req analysis.NewRunRequest) (analysis.Run, error) {
	if err := validateNewRunRequest(req); err != nil {
		return analysis.Run{}, err
	}

	resp, err := a.runClient.NewRun(ctx, callerID, req)
	if err != nil {
		return analysis.Run{}, fmt.Errorf("new run: client: %w", err)
	}

	_ = a.marketRepo.UpsertSymbols(ctx, req.Market.Exchange, req.Market.Category, []string{req.Market.Symbol})

	return resp, nil
}

func (a *Application) RunByID(ctx context.Context, runID uuid.UUID) (analysis.Run, error) {
	resp, err := a.runClient.RunByID(ctx, runID)
	if err != nil {
		return analysis.Run{}, fmt.Errorf("run by id: client: %w", err)
	}
	return resp, nil
}

func (a *Application) StreamRunArchive(ctx context.Context, runID uuid.UUID, dst io.Writer) error {
	return a.runClient.StreamRunArchive(ctx, runID, dst)
}

func (a *Application) ShareRun(ctx context.Context, callerID, runID uuid.UUID) error {
	if err := a.runClient.ShareRun(ctx, callerID, runID); err != nil {
		return fmt.Errorf("share run: client: %w", err)
	}
	return nil
}

func (a *Application) DeleteRun(ctx context.Context, callerID, runID uuid.UUID) error {
	if err := a.runClient.DeleteRun(ctx, callerID, runID); err != nil {
		return fmt.Errorf("delete run: client: %w", err)
	}
	return nil
}

func (a *Application) ListRunsPaged(ctx context.Context, callerID uuid.UUID, req analysis.ListRunsPagedRequest) (analysis.ListRunsPagedResponse, error) {
	if req.Limit <= 0 {
		return analysis.ListRunsPagedResponse{}, fmt.Errorf("limit: %w", errorsx.ErrInvalidArgument)
	}
	resp, err := a.runClient.ListRunsPaged(ctx, callerID, req)
	if err != nil {
		return analysis.ListRunsPagedResponse{}, fmt.Errorf("list runs paged: client: %w", err)
	}
	return resp, nil
}

func ParseRunFilter(raw string) (analysis.RunFilter, error) {
	switch strings.TrimSpace(raw) {
	case "", "mine":
		return analysis.RunFilterMine, nil
	case "shared":
		return analysis.RunFilterShared, nil
	case "all":
		return analysis.RunFilterAll, nil
	default:
		return analysis.RunFilterUnspecified, fmt.Errorf("filter: %w", errorsx.ErrInvalidArgument)
	}
}

func validateNewRunRequest(req analysis.NewRunRequest) error {
	if strings.TrimSpace(req.Market.Exchange) == "" {
		return fmt.Errorf("market.exchange: %w", errorsx.ErrInvalidArgument)
	}
	if strings.TrimSpace(req.Market.Category) == "" {
		return fmt.Errorf("market.category: %w", errorsx.ErrInvalidArgument)
	}
	if strings.TrimSpace(req.Market.Symbol) == "" {
		return fmt.Errorf("market.symbol: %w", errorsx.ErrInvalidArgument)
	}
	if strings.TrimSpace(req.Interval) == "" {
		return fmt.Errorf("interval: %w", errorsx.ErrInvalidArgument)
	}
	if strings.TrimSpace(req.Detector.Code) == "" {
		return fmt.Errorf("detector.code: %w", errorsx.ErrInvalidArgument)
	}
	if req.From.IsZero() || req.To.IsZero() {
		return fmt.Errorf("from/to: %w", errorsx.ErrInvalidArgument)
	}
	return nil
}

func (a *Application) ListAvailableExchanges(ctx context.Context) ([]exchange.Meta, error) {
	resp, err := a.catalogClient.ListAvailableExchanges(ctx)
	if err != nil {
		return nil, fmt.Errorf("list avaiable exchanges: %w", err)
	}
	return resp, nil
}

func (a *Application) ListAvailableDetectors(ctx context.Context) ([]detect.DetectorMeta, error) {
	resp, err := a.catalogClient.ListAvailableDetectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("list avaiable detectors: %w", err)
	}
	return resp, nil
}

func (a *Application) Info(ctx context.Context) (coresystem.ServiceInfo, error) {
	info, err := a.monitor.Info(ctx)
	if err != nil {
		return coresystem.ServiceInfo{}, fmt.Errorf("info: %w", err)
	}
	return info, nil
}

func (a *Application) Metrics(ctx context.Context) (coresystem.ServiceMetrics, error) {
	metrics, err := a.monitor.Metrics(ctx)
	if err != nil {
		return coresystem.ServiceMetrics{}, fmt.Errorf("metrics: %w", err)
	}
	return metrics, nil
}
