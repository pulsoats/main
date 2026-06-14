package core

import (
	"time"

	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/exchange"
	"github.com/pulsoats/core/market"
	"github.com/pulsoats/core/run"
)

func MarketSpecFromRequest(req MarketSpecRequest) market.Spec {
	return market.Spec{
		Exchange: req.Exchange,
		Category: req.Category,
		Symbol:   req.Symbol,
	}
}

func MarketSpecToResponse(spec market.Spec) MarketSpecResponse {
	return MarketSpecResponse{
		Exchange: spec.Exchange,
		Category: spec.Category,
		Symbol:   spec.Symbol,
	}
}

func DetectorConfigFromRequest(req DetectorConfigRequest) detect.DetectorConfig {
	return detect.DetectorConfig{
		Code:      req.Code,
		Version:   req.Version,
		OptsLabel: req.OptsLabel,
		Opts:      req.Opts,
	}
}

func DetectorConfigToResponse(cfg detect.DetectorConfig) DetectorConfigResponse {
	return DetectorConfigResponse{
		Code:      cfg.Code,
		Version:   cfg.Version,
		OptsLabel: cfg.OptsLabel,
		Opts:      cfg.Opts,
	}
}

func DetectorMetaToResponse(meta detect.DetectorMeta) DetectorMetaResponse {
	return DetectorMetaResponse{
		Code:        meta.Code,
		Kind:        string(meta.Kind),
		Version:     meta.Version,
		Description: meta.Description,
		OptsSchema:  meta.OptsSchema,
	}
}

func AvailableDetectorsToResponse(detectors []detect.DetectorMeta) AvailableDetectorsResponse {
	res := make([]DetectorMetaResponse, 0, len(detectors))
	for _, d := range detectors {
		res = append(res, DetectorMetaToResponse(d))
	}
	return AvailableDetectorsResponse{Detectors: res}
}

func RunStatusToResponse(status run.Status) RunStatusResponse {
	return RunStatusResponse{
		Code:    int(status.Code),
		Message: status.Message,
	}
}

func ExchangeMetaToResponse(meta exchange.Meta) ExchangeMetaResponse {
	return ExchangeMetaResponse{
		Code:       meta.Code,
		Intervals:  meta.Intervals,
		Categories: meta.Categories,
	}
}

func AvailableExchangesToResponse(exchanges []exchange.Meta) AvailableExchangesResponse {
	res := make([]ExchangeMetaResponse, 0, len(exchanges))
	for _, e := range exchanges {
		res = append(res, ExchangeMetaToResponse(e))
	}
	return AvailableExchangesResponse{Exchanges: res}
}

func BaseRunToResponse(base run.Base) BaseRunResponse {
	resp := BaseRunResponse{
		ID:           base.ID,
		Status:       RunStatusToResponse(base.Status),
		Market:       MarketSpecToResponse(base.Market),
		Interval:     base.Interval.String(),
		Detector:     DetectorConfigToResponse(base.Detector),
		SignalsCount: int(base.SignalsCount),
		CreatedBy:    base.CreatedBy,
		CreatedAt:    base.CreatedAt.Format(time.RFC3339),
	}
	if !base.FirstCandleTime.IsZero() {
		resp.FirstCandleTime = base.FirstCandleTime.Format(time.RFC3339)
	}
	if !base.LastCandleTime.IsZero() {
		resp.LastCandleTime = base.LastCandleTime.Format(time.RFC3339)
	}
	return resp
}
