package live

import (
	"github.com/google/uuid"
	"github.com/pulsoats/main/internal/transport/handlers/core"
)

type registerServiceRequest struct {
	Addr string `json:"addr" binding:"required"`
}

type newRunRequest struct {
	Market   core.MarketSpecRequest     `json:"market" binding:"required"`
	Interval string                     `json:"interval" binding:"required"`
	Detector core.DetectorConfigRequest `json:"detector" binding:"required"`
}

type listRunsQuery struct {
	Limit    int32      `form:"limit"`
	BeforeID *uuid.UUID `form:"beforeId"`
}

type listSignalsQuery struct {
	Limit    int32      `form:"limit"`
	BeforeID *uuid.UUID `form:"beforeId"`
	RunID    *uuid.UUID `form:"runId"`
}

type runResponse struct {
	core.BaseRunResponse
	OrdersCount      int64   `json:"ordersCount"`
	SumProfitPercent float64 `json:"sumProfitPercent"`
	FinishedAt       *string `json:"finishedAt,omitempty"`
	FinishedBy       *string `json:"finishedBy,omitempty"`
}

type listRunsResponse struct {
	Runs         []runResponse `json:"runs"`
	NextBeforeID *uuid.UUID    `json:"nextBeforeId,omitempty"`
	HasMore      bool          `json:"hasMore"`
}

type signalResponse struct {
	ID                uuid.UUID               `json:"id"`
	RunID             uuid.UUID               `json:"runId"`
	Market            core.MarketSpecResponse `json:"market"`
	DetectorCode      string                  `json:"detectorCode"`
	DetectorOptsLabel string                  `json:"detectorOptsLabel"`
	Time              int64                   `json:"time"`
	Value             int64                   `json:"value"`
	BuyValue          int64                   `json:"buyValue"`
	TakeProfitValue   int64                   `json:"takeProfitValue"`
	StopLossValue     int64                   `json:"stopLossValue"`
	ExpectedReturnPPM int64                   `json:"expectedReturnPpm"`
	Fingerprint       uuid.UUID               `json:"fingerprint"`
	CreatedAt         int64                   `json:"createdAt"`
}

type listSignalsResponse struct {
	Signals      []signalResponse `json:"signals"`
	NextBeforeID *uuid.UUID       `json:"nextBeforeId,omitempty"`
	HasMore      bool             `json:"hasMore"`
}

type listServicesResponse struct {
	Services []core.ServiceInfoResponse `json:"services"`
}
