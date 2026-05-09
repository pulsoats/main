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
	OrdersCount      int64
	SumProfitPercent float64
	FinishedAt       *time.Time
	FinishedBy       *string
}

type ListRunsRequest struct {
	Limit    int32
	BeforeID *uuid.UUID
	Filter   *ListRunsFilter
}

type ListRunsResponse struct {
	Runs         []Run
	NextBeforeID *uuid.UUID
	HasMore      bool
}

type ListRunsFilter struct {
	StatusCode   *corerun.StatusCode
	Category     *string
	Symbol       *string
	Interval     *market.Interval
	DetectorCode *string
	OrderDirAsc  *bool
}

type ListSignalsPagedRequest struct {
	Limit    int32
	BeforeID *uuid.UUID
	Filter   *ListSignalsFilter
}

type ListSignalsPagedResponse struct {
	Signals      []detect.Signal
	NextBeforeID *uuid.UUID
	HasMore      bool
}

type ListSignalsFilter struct {
	RunID       *uuid.UUID
	Category    *string
	Symbol      *string
	From        *time.Time
	To          *time.Time
	OrderDirAsc *bool
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
	Signal detect.Signal
}

func (RunEvent) eventPayload()    {}
func (SignalEvent) eventPayload() {}

type Stream struct {
	Events <-chan Event
	Err    error
}
