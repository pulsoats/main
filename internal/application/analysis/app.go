package analysis

import (
	"context"
	"fmt"
	"io"

	"github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/main/internal/domain/analysis"
)

type marketApp interface {
	EnsureMarket(ctx context.Context, spec market.Spec) error
}

type Application struct {
	client    analysis.Client
	marketApp marketApp
}

func NewApplication(client analysis.Client, marketApplication marketApp) *Application {
	return &Application{client: client, marketApp: marketApplication}
}

func (s *Application) StartRun(ctx context.Context, req analysis.StartRunRequest) (string, error) {
	err := req.Validate()
	if err != nil {
		return "", fmt.Errorf("start run: %w", err)
	}

	if err = s.marketApp.EnsureMarket(ctx, req.Market); err != nil {
		return "", fmt.Errorf("start run: %w", err)
	}

	runID, err := s.client.StartRun(ctx, req)
	if err != nil {
		return "", fmt.Errorf("start run: %w", err)
	}
	return runID, nil
}

func (s *Application) RunStatus(ctx context.Context, runID string) (analysis.RunStatus, error) {
	status, err := s.client.GetRunStatus(ctx, runID)
	if err != nil {
		return analysis.RunStatus{}, fmt.Errorf("run status: %w", err)
	}
	return status, nil
}

func (s *Application) RunMeta(ctx context.Context, runID string) (analysis.Run, error) {
	meta, err := s.client.GetRunMeta(ctx, runID)
	if err != nil {
		return analysis.Run{}, fmt.Errorf("run meta: %w", err)
	}
	return meta, nil
}

func (s *Application) RunResult(ctx context.Context, runID string) (analysis.RunResultArchive, error) {
	stream, err := s.client.GetRunResult(ctx, runID)
	if err != nil {
		return analysis.RunResultArchive{}, fmt.Errorf("run result: %w", err)
	}

	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		for {
			select {
			case data, ok := <-stream:
				if !ok {
					return
				}

				if _, err := pw.Write(data); err != nil {
					return
				}
			case <-ctx.Done():
				pw.CloseWithError(ctx.Err())
				return
			}
		}
	}()

	return analysis.RunResultArchive{
		Filename: "run_" + runID + ".zip",
		Content:  pr,
	}, nil
}

func (s *Application) ListRunsPaged(ctx context.Context, limit int, beforeID *int64) (analysis.RunsPage, error) {
	page, err := s.client.ListRunsPaged(ctx, limit, beforeID)
	if err != nil {
		return analysis.RunsPage{}, fmt.Errorf("list runs: %w", err)
	}
	return page, nil
}
