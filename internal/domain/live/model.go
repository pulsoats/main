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

type RunsRequest struct {
	Limit    int32
	BeforeID *uuid.UUID
	Filter   *RunsFilter
}

type RunsResponse struct {
	Runs         []Run
	NextBeforeID *uuid.UUID
	HasMore      bool
}

type RunsFilter struct {
	StatusCode   *corerun.StatusCode
	Category     *string
	Symbol       *string
	Interval     *market.Interval
	DetectorCode *string
	OrderDirAsc  *bool
}

type SignalsPagedRequest struct {
	Limit    int32
	BeforeID *uuid.UUID
	Filter   *SignalsFilter
}

type SignalsPagedResponse struct {
	Signals      []detect.Signal
	NextBeforeID *uuid.UUID
	HasMore      bool
}

type SignalsFilter struct {
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

