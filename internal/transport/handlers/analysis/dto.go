package analysis

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/main/internal/domain/analysis"
)

type startRunRequest struct {
	Market    marketSpecRequest     `json:"market" binding:"required"`
	Interval  string                `json:"interval" binding:"required"`
	From      string                `json:"fromTime" binding:"required"` //  ISO‑8601 | RFC3339
	To        string                `json:"toTime" binding:"required"`   //  ISO‑8601 | RFC3339
	PriceType string                `json:"priceType" binding:"required"`
	Detector  detectorConfigRequest `json:"detector" binding:"required"`
	Fees      *feesRequest          `json:"fees"`
}

type marketSpecRequest struct {
	Exchange string `json:"exchange" binging:"required"`
	Category string `json:"category" binding:"required"`
	Symbol   string `json:"symbol" binding:"required"`
}

func mapToMarketSpec(req marketSpecRequest) market.Spec {
	return market.Spec{
		Exchange: req.Exchange,
		Category: market.Category(req.Category),
		Symbol:   strings.ToUpper(req.Symbol),
	}
}

type marketSpecResponse struct {
	Exchange string `json:"exchange"`
	Category string `json:"category"`
	Symbol   string `json:"symbol"`
}

func mapToMarketSpecResponse(spec market.Spec) marketSpecResponse {
	return marketSpecResponse{
		Exchange: spec.Exchange,
		Category: string(spec.Category),
		Symbol:   spec.Symbol,
	}
}

type detectorConfigRequest struct {
	Code  string          `json:"code" binding:"required"`
	Label string          `json:"label"`
	Opts  json.RawMessage `json:"opts"`
}

func mapToDetectorConfig(req detectorConfigRequest) detect.DetectorConfig {
	return detect.DetectorConfig{
		Code:  req.Code,
		Label: req.Label,
		Opts:  req.Opts,
	}
}

type detectorConfigResponse struct {
	Code  string          `json:"code"`
	Label string          `json:"label"`
	Opts  json.RawMessage `json:"opts"`
}

func mapToDetectorConfigResponse(cfg detect.DetectorConfig) detectorConfigResponse {
	return detectorConfigResponse{
		Code:  cfg.Code,
		Label: cfg.Label,
		Opts:  cfg.Opts,
	}
}

type feesRequest struct {
	TakerFee float64 `json:"takerFee" binding:"required"`
	MakerFee float64 `json:"makerFee" binding:"required"`
}

func mapToFees(req *feesRequest) *market.TakerMakerFees {
	if req == nil {
		return nil
	}

	takerPPM := int64(1_000_000 * req.TakerFee)
	makerPPM := int64(1_000_000 * req.MakerFee)
	return &market.TakerMakerFees{
		TakerFeeRate: takerPPM,
		MakerFeeRate: makerPPM,
	}
}

type startRunResponse struct {
	RunID string `json:"runId"`
}

type runStatusInfo struct {
	Code    int    `json:"code"`
	Name    string `json:"name"`
	Message string `json:"message,omitempty"`
}

type runMetaResponse struct {
	ID           string                 `json:"id"`
	Status       runStatusInfo          `json:"status"`
	Market       marketSpecResponse     `json:"market"`
	PriceType    string                 `json:"priceType"`
	Interval     string                 `json:"interval"`
	From         string                 `json:"fromTime"`
	To           string                 `json:"toTime"`
	Detector     detectorConfigResponse `json:"detector"`
	SignalsCount string                 `json:"signalsCount"`
	AvgProfit    float64                `json:"avgProfit"`
	CreatedBy    string                 `json:"createdBy"`
	CreatedAt    string                 `json:"createdAt"`
	IsShared     bool                   `json:"isShared"`
	SharedAt     *string                `json:"sharedAt,omitempty"`
}

func mapToRunMetaResponse(meta analysis.Run) runMetaResponse {
	var sharedAt *string
	if meta.SharedAt != nil {
		s := meta.SharedAt.Format(time.RFC3339)
		sharedAt = &s
	}

	return runMetaResponse{
		ID: meta.ID,
		Status: runStatusInfo{
			Code:    meta.Status.Code,
			Name:    analysis.StatusName(meta.Status.Code),
			Message: meta.Status.Message,
		},
		Market:       mapToMarketSpecResponse(meta.Market),
		PriceType:    meta.PriceType,
		Interval:     meta.Interval.String(),
		From:         meta.From.Format(time.RFC3339),
		To:           meta.To.Format(time.RFC3339),
		Detector:     mapToDetectorConfigResponse(meta.Detector),
		SignalsCount: strconv.FormatInt(meta.SignalsCount, 10),
		AvgProfit:    float64(meta.AvgProfitPPM) / 1_000_000,
		CreatedBy:    meta.CreatedBy,
		CreatedAt:    meta.CreatedAt.Format(time.RFC3339),
		IsShared:     meta.IsShared,
		SharedAt:     sharedAt,
	}
}

type runListResponse struct {
	Items        []runMetaResponse `json:"items"`
	NextBeforeID *int64            `json:"nextBeforeId,omitempty"`
	HasMore      bool              `json:"hasMore"`
}

func mapRunsPageToResponse(page analysis.RunsPage) runListResponse {
	items := make([]runMetaResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, mapToRunMetaResponse(item))
	}
	return runListResponse{
		Items:        items,
		NextBeforeID: page.NextBeforeID,
		HasMore:      page.HasMore,
	}
}
