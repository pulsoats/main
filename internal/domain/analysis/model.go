package analysis

import (
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/market"
	"github.com/pulsoats/core/run"
)

type Run struct {
	BaseRun         run.Base
	Fees            market.TakerMakerFees
	SumProfitPPM    int64
	AvgProfitPPM    int64
	DisableStopLoss bool
	DisableRepeats  bool
	IsShared        bool
	SharedAt        *time.Time
}

type NewRunRequest struct {
	Market          market.Spec
	Interval        string
	From            time.Time
	To              time.Time
	Detector        detect.DetectorConfig
	Fees            *market.TakerMakerFees
	DisableStopLoss bool
	DisableRepeats  bool
}

type RunsFilter struct {
	Exchanges     []string
	Categories    []string
	Symbols       []string
	Intervals     []string
	DetectorCodes []string
	Statuses      []int

	MinSignals      *int64
	MaxSignals      *int64
	MinAvgProfitPPM *int64
	MaxAvgProfitPPM *int64

	DisableStopLoss *bool
	DisableRepeats  *bool

	FirstCandleFrom *time.Time
	LastCandleTo    *time.Time

	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

type RunsPagedRequest struct {
	Limit       int32
	BeforeID    *uuid.UUID
	OrderDirAsc bool
	Scope       int
	Filter      *RunsFilter
}

type RunsPagedResponse struct {
	Runs         []Run
	HasMore      bool
	NextBeforeID *uuid.UUID
}
