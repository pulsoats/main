package analysis

import (
	"context"
	"fmt"
	"io"

	"github.com/pulsoats/main/internal/domain/analysis"
	"github.com/pulsoats/main/internal/domain/market"
)

type service struct {
	client    analysis.Client
	marketSvc market.Service
}

type ServiceConfig struct {
	Client       analysis.Client
	MarketHelper market.Service
}

func NewService(client analysis.Client, marketService market.Service) analysis.Service {
	return &service{client: client, marketSvc: marketService}
}

func (s *service) StartRun(ctx context.Context, req analysis.StartRunRequest) (string, error) {
	err := req.Validate()
	if err != nil {
		return "", fmt.Errorf("start run: %w", err)
	}

	if err = s.marketSvc.EnsureMarket(ctx, req.Market); err != nil {
		return "", fmt.Errorf("start run: %w", err)
	}

	runID, err := s.client.StartRun(ctx, req)
	if err != nil {
		return "", fmt.Errorf("start run: %w", err)
	}
	return runID, nil
}

func (s *service) RunStatus(ctx context.Context, runID string) (analysis.RunStatus, error) {
	status, err := s.client.GetRunStatus(ctx, runID)
	if err != nil {
		return analysis.RunStatus{}, fmt.Errorf("run status: %w", err)
	}
	return status, nil
}

func (s *service) RunMeta(ctx context.Context, runID string) (analysis.Run, error) {
	meta, err := s.client.GetRunMeta(ctx, runID)
	if err != nil {
		return analysis.Run{}, fmt.Errorf("run meta: %w", err)
	}
	return meta, nil
}

func (s *service) RunResult(ctx context.Context, runID string) (analysis.RunResultArchive, error) {
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
