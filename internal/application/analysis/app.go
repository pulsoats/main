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
	"github.com/pulsoats/main/internal/domain/analysis"
	"github.com/pulsoats/main/internal/domain/market"
	grpcanalysis "github.com/pulsoats/main/internal/infrastructure/grpc/analysis"
)

type Application struct {
	client     *grpcanalysis.Client
	marketRepo market.Repository
}

func NewApplication(client *grpcanalysis.Client, marketRepo market.Repository) (*Application, error) {
	const op = "analysis app"
	if client == nil {
		return nil, fmt.Errorf("%s: client is nil", op)
	}
	return &Application{client: client, marketRepo: marketRepo}, nil
}

func (a *Application) NewRun(ctx context.Context, userID uuid.UUID, req analysis.NewRunRequest) (analysis.Run, error) {
	const op = "new run"
	if err := validateNewRunRequest(req); err != nil {
		return analysis.Run{}, err
	}

	resp, err := a.client.NewRun(ctx, userID, req)
	if err != nil {
		return analysis.Run{}, fmt.Errorf("%s: client: %w", op, err)
	}

	_ = a.marketRepo.UpsertSymbols(ctx, req.Market.Exchange, req.Market.Category, []string{req.Market.Symbol})

	return resp, nil
}

func (a *Application) RunByID(ctx context.Context, runID uuid.UUID) (analysis.Run, error) {
	const op = "run by id"
	resp, err := a.client.RunByID(ctx, runID)
	if err != nil {
		return analysis.Run{}, fmt.Errorf("%s: client: %w", op, err)
	}
	return resp, nil
}

func (a *Application) StreamRunArchive(ctx context.Context, runID uuid.UUID, dst io.Writer) error {
	return a.client.StreamRunArchive(ctx, runID, dst)
}

func (a *Application) ShareRun(ctx context.Context, userID, runID uuid.UUID) error {
	const op = "share run"
	if err := a.client.ShareRun(ctx, userID, runID); err != nil {
		return fmt.Errorf("%s: client: %w", op, err)
	}
	return nil
}

func (a *Application) DeleteRun(ctx context.Context, userID, runID uuid.UUID) error {
	const op = "delete run"
	if err := a.client.DeleteRun(ctx, userID, runID); err != nil {
		return fmt.Errorf("%s: client: %w", op, err)
	}
	return nil
}

func (a *Application) AvailableExchanges(ctx context.Context) ([]exchange.Meta, error) {
	const op = "available exchanges"
	resp, err := a.client.AvailableExchanges(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return resp, nil
}

func (a *Application) AvailableDetectors(ctx context.Context) ([]detect.DetectorMeta, error) {
	const op = "available detectors"
	resp, err := a.client.AvailableDetectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return resp, nil
}

func (a *Application) RunsPaged(ctx context.Context, userID uuid.UUID, req analysis.RunsPagedRequest) (analysis.RunsPagedResponse, error) {
	resp, err := a.client.RunsPaged(ctx, userID, req)
	if err != nil {
		return analysis.RunsPagedResponse{}, fmt.Errorf("runs paged: client: %w", err)
	}
	return resp, nil
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
