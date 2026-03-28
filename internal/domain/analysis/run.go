package analysis

import (
	"time"

	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/market"
)

const (
	StatusUnspecified = 0
	StatusPending     = 1
	StatusRunning     = 2
	StatusDone        = 3
	StatusFailed      = 4
)

var runStatusNames = map[int]string{
	StatusUnspecified: "unspecified",
	StatusPending:     "pending",
	StatusRunning:     "running",
	StatusDone:        "done",
	StatusFailed:      "failed",
}

type Run struct {
	ID           string
	Status       RunStatus
	Market       market.Spec
	PriceType    string
	Interval     market.Interval
	From         time.Time
	To           time.Time
	Detector     detect.DetectorConfig
	SignalsCount int64
	AvgProfitPPM int64
	CreatedBy    string
	CreatedAt    time.Time
	IsShared     bool
	SharedAt     *time.Time
}

type RunStatus struct {
	Code    int
	Message string
}

func (s RunStatus) Name() string {
	return StatusName(s.Code)
}

func StatusName(code int) string {
	if name, ok := runStatusNames[code]; ok {
		return name
	}
	return "unknown"
}

type RunsPage struct {
	Items        []Run
	NextBeforeID *int64
	HasMore      bool
}
