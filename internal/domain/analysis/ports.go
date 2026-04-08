package analysis

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/errorsx"
)

type Client interface {
	StartRun(ctx context.Context, req StartRunRequest) (string, error)
	GetRunMeta(ctx context.Context, runID string) (Run, error)
	GetRunResult(ctx context.Context, runID string) (chan []byte, error)
	ListRunsPaged(ctx context.Context, userID uuid.UUID, limit int, beforeID *int64, filter string) (RunsPage, error)
	ShareRun(ctx context.Context, userID uuid.UUID, runID string) error
	DeleteRun(ctx context.Context, userID uuid.UUID, runID string) error
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
		return fmt.Errorf("start run request validation: user id: %w", errorsx.ErrInvalidArgument)
	}

	if r.Market == (market.Spec{}) {
		return fmt.Errorf("start run request validation: market specs: %w", errorsx.ErrInvalidArgument)
	}

	if r.From.IsZero() || r.To.IsZero() {
		return fmt.Errorf("start run request validation: time can't be zero: %w", errorsx.ErrInvalidArgument)
	}
	if r.From.After(r.To) {
		return fmt.Errorf("start run request validation: from can't be after to: %w", errorsx.ErrInvalidArgument)
	}

	if r.Detector.Code == "" {
		return fmt.Errorf("start run request validation: detector code: %w", errorsx.ErrInvalidArgument)
	}
	return nil
}

type RunResultArchive struct {
	Filename string
	Content  io.ReadCloser
}
