package analysis

import (
	"github.com/google/uuid"
	"github.com/pulsoats/main/internal/transport/handlers/core"
)

type newRunRequest struct {
	Market   core.MarketSpecRequest     `json:"market" binding:"required"`
	Interval string                     `json:"interval" binding:"required"`
	FromTime string                     `json:"fromTime" binding:"required"`
	ToTime   string                     `json:"toTime" binding:"required"`
	Detector core.DetectorConfigRequest `json:"detector" binding:"required"`
	Fees     *feesRequest               `json:"fees,omitempty"`
}

type feesRequest struct {
	TakerFee float64 `json:"takerFee" binding:"required"`
	MakerFee float64 `json:"makerFee" binding:"required"`
}

type runResponse struct {
	core.BaseRunResponse
	AvgProfitPercent float64   `json:"avgProfitPercent"`
	IsShared         bool      `json:"isShared"`
	SharedAt         *string   `json:"sharedAt"`
	ServiceID        uuid.UUID `json:"serviceId"`
}

type listRunsResponse struct {
	Runs         []runResponse `json:"runs"`
	NextBeforeID *uuid.UUID    `json:"nextBeforeId,omitempty"`
	HasMore      bool          `json:"hasMore"`
}
