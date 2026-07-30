package live

import (
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/detect/detector"
	"github.com/pulsoats/core/detect/filter"
	"github.com/pulsoats/core/market"
	corerun "github.com/pulsoats/core/run"
)

type Run struct {
	corerun.Base
	FinishedAt *time.Time
	FinishedBy *uuid.UUID
}

type NewRunRequest struct {
	MarketSpec     market.Spec
	Interval       string
	DetectorConfig detector.Config
	FiltersConfigs []filter.Config
}

type RunsFilter struct {
	Categories    []string
	Symbols       []string
	Intervals     []string
	DetectorCodes []string
	Statuses      []int

	MinSignals *int64
	MaxSignals *int64

	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

type RunsPagedRequest struct {
	Limit       int32
	BeforeID    *uuid.UUID
	OrderDirAsc bool
	Filter      *RunsFilter
}

type RunsPagedResponse struct {
	Runs         []Run
	NextBeforeID *uuid.UUID
	HasMore      bool
}

type SignalsFilter struct {
	RunID         *uuid.UUID
	Categories    []string
	Symbols       []string
	Intervals     []string
	DetectorCodes []string
	CreatedFrom   *time.Time
	CreatedTo     *time.Time
}

type SignalsPagedRequest struct {
	Limit       int32
	BeforeID    *uuid.UUID
	OrderDirAsc bool
	Filter      *SignalsFilter
}

// EnrichedSignal дополняет detect.Signal данными прогона (market, interval),
// полученными из proto-ответа live-сервиса.
type EnrichedSignal struct {
	detect.Signal
	Market          market.Spec
	Interval        string
	DetectorCode    string
	DetectorVersion string
}

type SignalsPagedResponse struct {
	Signals      []EnrichedSignal
	HasMore      bool
	NextBeforeID *uuid.UUID
}

type Event struct {
	Payload EventPayload
}

type EventPayload interface {
	eventPayload()
}

type RunStatusEvent struct {
	RunID  uuid.UUID
	Status corerun.Status
}

type SignalEvent struct {
	Signal EnrichedSignal
}

func (RunStatusEvent) eventPayload() {}
func (SignalEvent) eventPayload()    {}
