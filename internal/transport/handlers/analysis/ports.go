package analysis

import (
	"context"

	"github.com/pulsoats/main/internal/domain/analysis"
)

type app interface {
	StartRun(ctx context.Context, req analysis.StartRunRequest) (string, error)
	RunStatus(ctx context.Context, runID string) (analysis.RunStatus, error)
	RunMeta(ctx context.Context, runID string) (analysis.Run, error)
	RunResult(ctx context.Context, runID string) (analysis.RunResultArchive, error)
	ListRunsPaged(ctx context.Context, limit int, beforeID *int64) (analysis.RunsPage, error)
}
