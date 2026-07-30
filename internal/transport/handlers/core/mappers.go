package core

import (
	"time"

	"github.com/pulsoats/core/detect/detector"
	"github.com/pulsoats/core/detect/filter"
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

func DetectorConfigFromRequest(req DetectorConfigRequest) detector.Config {
	return detector.Config{
		Code:      req.Code,
		Version:   req.Version,
		OptsLabel: req.OptsLabel,
		Opts:      req.Opts,
	}
}

func DetectorConfigToResponse(dc detector.Config) DetectorConfigResponse {
	return DetectorConfigResponse{
		Code:      dc.Code,
		Version:   dc.Version,
		OptsLabel: dc.OptsLabel,
		Opts:      dc.Opts,
	}
}

func DetectorMetaToResponse(meta detector.Meta) DetectorMetaResponse {
	return DetectorMetaResponse{
		Code:        meta.Code,
		Version:     meta.Version,
		Description: meta.Description,
		OptsSchema:  meta.OptsSchema,
	}
}

func AvailableDetectorsToResponse(detectors []detector.Meta) AvailableDetectorsResponse {
	res := make([]DetectorMetaResponse, 0, len(detectors))
	for _, d := range detectors {
		res = append(res, DetectorMetaToResponse(d))
	}
	return AvailableDetectorsResponse{Detectors: res}
}

func FilterConfigFromRequest(req FilterConfigRequest) filter.Config {
	return filter.Config{
		Code:   req.Code,
		Period: req.Period,
	}
}

func FilterConfigToResponse(fc filter.Config) FilterConfigResponse {
	return FilterConfigResponse{
		Code:   fc.Code,
		Period: fc.Period,
	}
}

func FiltersConfigsFromRequest(req []FilterConfigRequest) []filter.Config {
	out := make([]filter.Config, 0, len(req))
	for _, fc := range req {
		out = append(out, FilterConfigFromRequest(fc))
	}
	return out
}

func FiltersConfigsToResponse(cfgs []filter.Config) []FilterConfigResponse {
	out := make([]FilterConfigResponse, 0, len(cfgs))
	for _, fc := range cfgs {
		out = append(out, FilterConfigToResponse(fc))
	}
	return out
}

func AvailableFiltersToResponse(filters []filter.Meta) AvailableFiltersResponse {
	out := make([]FilterMetaResponse, 0, len(filters))
	for _, m := range filters {
		out = append(out, FilterMetaResponse{Code: m.Code, Description: m.Description})
	}
	return AvailableFiltersResponse{
		Filters: out,
	}
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
		ID:             base.ID,
		Status:         RunStatusToResponse(base.Status),
		Market:         MarketSpecToResponse(base.Market),
		Interval:       base.Interval.String(),
		DetectorConfig: DetectorConfigToResponse(base.DetectorConfig),
		FiltersConfigs: FiltersConfigsToResponse(base.FiltersConfigs),
		SignalsCount:   int(base.SignalsCount),
		CreatedBy:      base.CreatedBy,
		CreatedAt:      base.CreatedAt.Format(time.RFC3339),
	}
	if !base.FirstCandleTime.IsZero() {
		resp.FirstCandleTime = base.FirstCandleTime.Format(time.RFC3339)
	}
	if !base.LastCandleTime.IsZero() {
		resp.LastCandleTime = base.LastCandleTime.Format(time.RFC3339)
	}
	return resp
}
