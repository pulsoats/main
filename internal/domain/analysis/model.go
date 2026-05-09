package analysis

import (
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/market"
	"github.com/pulsoats/core/run"
)

type NewRunRequest struct {
	Market   market.Spec
	Interval string
	From     time.Time
	To       time.Time
	Detector detect.DetectorConfig
	Fees     *market.TakerMakerFees
}

type Run struct {
	BaseRun          run.Base
	AvgProfitPercent float64
	IsShared         bool
	SharedAt         *time.Time
}

type RunFilter int32

const (
	RunFilterUnspecified RunFilter = 0
	RunFilterMine        RunFilter = 1
	RunFilterShared      RunFilter = 2
	RunFilterAll         RunFilter = 3
)

type ListRunsPagedRequest struct {
	Limit    int32
	BeforeID *uuid.UUID
	Filter   RunFilter
}

type ListRunsPagedResponse struct {
	Runs         []Run
	NextBeforeID *uuid.UUID
	HasMore      bool
}
