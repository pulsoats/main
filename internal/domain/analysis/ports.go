package analysis

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/market"
)

type Client interface {
	StartRun(ctx context.Context, req StartRunRequest) (string, error)
	GetRunStatus(ctx context.Context, runID string) (RunStatus, error)
	GetRunMeta(ctx context.Context, runID string) (Run, error)
	GetRunResult(ctx context.Context, runID string) (chan []byte, error)
	ListRunsPaged(ctx context.Context, limit int, beforeID *int64) (RunsPage, error)
}
type Service interface {
	StartRun(ctx context.Context, req StartRunRequest) (string, error)
	RunStatus(ctx context.Context, runID string) (RunStatus, error)
	RunMeta(ctx context.Context, runID string) (Run, error)
	RunResult(ctx context.Context, runID string) (RunResultArchive, error)
	ListRunsPaged(ctx context.Context, limit int, beforeID *int64) (RunsPage, error)
}

type StartRunRequest struct {
	UserID    string
	Market    market.Spec
	Interval  market.Interval
	From      time.Time
	To        time.Time
	PriceType market.PriceType
	Detector  detect.DetectorConfig
	Fees      *market.TakerMakerFees
}

func (r *StartRunRequest) Validate() error {
	if r.UserID == "" {
		return fmt.Errorf("start run request validation: %w: user id", derrors.ErrRequired)
	}

	if r.Market == (market.Spec{}) {
		return fmt.Errorf("start run request validation: %w: market specs", derrors.ErrRequired)
	}

	if r.From.IsZero() || r.To.IsZero() {
		return fmt.Errorf("start run request validation: %w: time can't be zero", derrors.ErrRequired)
	}
	if r.From.After(r.To) {
		return fmt.Errorf("start run request validation: %w: from can't be after to", derrors.ErrInvalidArgument)
	}

	if r.Detector.Code == "" {
		return fmt.Errorf("start run request validation: %w: detector code", derrors.ErrRequired)
	}
	return nil
}

type RunResultArchive struct {
	Filename string
	Content  io.ReadCloser
}
