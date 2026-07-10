package live

import (
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/market"
	corerun "github.com/pulsoats/core/run"
)

type Run struct {
	corerun.Base
	SumProfitPPM int64
	FinishedAt   *time.Time
	FinishedBy   *uuid.UUID
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
	Market   market.Spec
	Interval market.Interval
}

type SignalsPagedResponse struct {
	Signals      []EnrichedSignal
	HasMore      bool
	NextBeforeID *uuid.UUID
}

type Event struct {
	RunID   uuid.UUID
	Payload EventPayload
}

type EventPayload interface {
	eventPayload()
}

type RunEvent struct {
	Run Run
}

type SignalEvent struct {
	Signal EnrichedSignal
}

func (RunEvent) eventPayload()    {}
func (SignalEvent) eventPayload() {}
