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
	From      string                `json:"from_time" binding:"required"` //  ISO‑8601 | RFC3339
	To        string                `json:"to_time" binding:"required"`   //  ISO‑8601 | RFC3339
	PriceType string                `json:"price_type" binding:"required"`
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
	TakerFee float64 `json:"taker_fee" binding:"required"`
	MakerFee float64 `json:"maker_fee" binding:"required"`
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
	RunID string `json:"run_id"`
}

type runStatusResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type runMetaResponse struct {
	ID           string                 `json:"id"`
	Market       marketSpecResponse     `json:"market"`
	Interval     string                 `json:"interval"`
	Detector     detectorConfigResponse `json:"detector"`
	From         string                 `json:"from_time"` //  ISO‑8601 | RFC3339
	To           string                 `json:"to_time"`   //  ISO‑8601 | RFC3339
	SignalsCount string                 `json:"signals_count"`
	AvgProfit    float64                `json:"avg_profit"`
	CreatedAt    string                 `json:"created_at"` // ISO‑8601 | RFC3339
}

func mapToRunMetaResponse(meta analysis.Run) runMetaResponse {
	avgProfitFloat := float64(meta.AvgProfitPPM) / 1_000_000
	return runMetaResponse{
		ID:           meta.ID,
		Market:       mapToMarketSpecResponse(meta.Market),
		Interval:     meta.Interval.String(),
		Detector:     mapToDetectorConfigResponse(meta.Detector),
		From:         meta.From.Format(time.RFC3339),
		To:           meta.To.Format(time.RFC3339),
		SignalsCount: strconv.FormatInt(meta.SignalsCount, 10),
		AvgProfit:    avgProfitFloat,
		CreatedAt:    meta.CreatedAt.Format(time.RFC3339),
	}
}
