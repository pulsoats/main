package analysis

import (
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/main/internal/transport/handlers/core"
)

type newRunRequest struct {
	Market           core.MarketSpecRequest     `json:"market" binding:"required"`
	Interval         string                     `json:"interval" binding:"required"`
	FromTime         string                     `json:"fromTime" binding:"required"`
	ToTime           string                     `json:"toTime" binding:"required"`
	DetectorConfig   core.DetectorConfigRequest `json:"detectorConfig" binding:"required"`
	FiltersConfigs   []core.FilterConfigRequest `json:"filtersConfigs"`
	Fees             *feesRequest               `json:"fees,omitempty"`
	DisableStopLoss  bool                       `json:"disableStopLoss"`
	DisableRepeats   bool                       `json:"disableRepeats"`
	CollectRejectLog bool                       `json:"collectRejectLog"`
}

type feesRequest struct {
	TakerFeePct float64 `json:"takerFeePct" binding:"required"`
	MakerFeePct float64 `json:"makerFeePct" binding:"required"`
}

type feesResponse struct {
	TakerFeePct float64 `json:"takerFeePct"`
	MakerFeePct float64 `json:"makerFeePct"`
}

type runResponse struct {
	core.BaseRunResponse
	Fees            feesResponse `json:"fees"`
	SumProfitPct    float64      `json:"sumProfitPct"`
	AvgProfitPct    float64      `json:"avgProfitPct"`
	DisableStopLoss bool         `json:"disableStopLoss"`
	DisableRepeats  bool         `json:"disableRepeats"`
	IsShared        bool         `json:"isShared"`
	SharedAt        *string      `json:"sharedAt"`
}

type runsPagedRequest struct {
	Limit       int32  `form:"limit"`
	BeforeID    string `form:"before_id"`
	OrderDirAsc bool   `form:"order_dir_asc"`
	Scope       int    `form:"scope" binding:"required,oneof=1 2 3"`

	Exchanges     []string `form:"exchanges"`
	Categories    []string `form:"categories"`
	Symbols       []string `form:"symbols"`
	Intervals     []string `form:"intervals"`
	DetectorCodes []string `form:"detector_codes"`
	Statuses      []int    `form:"statuses"`

	MinSignals *int64 `form:"min_signals"`
	MaxSignals *int64 `form:"max_signals"`

	MinAvgProfitPct *float64 `form:"min_avg_profit_pct"`
	MaxAvgProfitPct *float64 `form:"max_avg_profit_pct"`

	DisableStopLoss *bool `form:"disable_stop_loss"`
	DisableRepeats  *bool `form:"disable_repeats"`

	FirstCandleFrom *time.Time `form:"first_candle_from"` // RFC3339 по умолчанию
	LastCandleTo    *time.Time `form:"last_candle_to"`
	CreatedFrom     *time.Time `form:"created_from"`
	CreatedTo       *time.Time `form:"created_to"`
}

type runsPagedResponse struct {
	Runs         []runResponse `json:"runs"`
	HasMore      bool          `json:"hasMore"`
	NextBeforeID *uuid.UUID    `json:"nextBeforeId,omitempty"`
}
