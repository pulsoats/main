package analysis

import (
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/market"
	"github.com/pulsoats/core/run"
)

type Run struct {
	BaseRun          run.Base
	Fees             market.TakerMakerFees
	SumProfitPercent float64
	AvgProfitPercent float64
	IsShared         bool
	SharedAt         *time.Time
}

type NewRunRequest struct {
	Market   market.Spec
	Interval string
	From     time.Time
	To       time.Time
	Detector detect.DetectorConfig
	Fees     *market.TakerMakerFees
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
	MinAvgProfitPct *float64
	MaxAvgProfitPct *float64

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
