package core

import (
	"strconv"
	"time"

	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/exchange"
	"github.com/pulsoats/core/market"
	"github.com/pulsoats/core/run"
	coresystem "github.com/pulsoats/core/system"
	domainsystem "github.com/pulsoats/main/internal/domain/system"
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
		OptsLabel: req.OptsLabel,
		Opts:      req.Opts,
	}
}

func DetectorConfigToResponse(cfg detect.DetectorConfig) DetectorConfigResponse {
	return DetectorConfigResponse{
		Code:      cfg.Code,
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

func ListAvailableDetectorsToResponse(detectors []detect.DetectorMeta) ListAvailableDetectorsResponse {
	res := make([]DetectorMetaResponse, 0, len(detectors))
	for _, d := range detectors {
		res = append(res, DetectorMetaToResponse(d))
	}
	return ListAvailableDetectorsResponse{Detectors: res}
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

func ListAvailableExchangesToResponse(exchanges []exchange.Meta) ListAvailableExchangesResponse {
	res := make([]ExchangeMetaResponse, 0, len(exchanges))
	for _, e := range exchanges {
		res = append(res, ExchangeMetaToResponse(e))
	}
	return ListAvailableExchangesResponse{Exchanges: res}
}

func BaseRunToResponse(base run.Base) BaseRunResponse {
	return BaseRunResponse{
		ID:           base.ID,
		Status:       RunStatusToResponse(base.Status),
		Market:       MarketSpecToResponse(base.Market),
		Interval:     base.Interval.String(),
		Detector:     DetectorConfigToResponse(base.Detector),
		SignalsCount: int(base.SignalsCount),
		CreatedBy:    base.CreatedBy,
		CreatedAt:    base.CreatedAt.Format(time.RFC3339),
	}
}

// ServiceInfoToResponse маппит core ServiceInfo (из gRPC monitor) — без Addr и timestamps.
func ServiceInfoToResponse(info coresystem.ServiceInfo) ServiceInfoResponse {
	return ServiceInfoResponse{
		ID:       info.ID,
		Kind:     string(info.Kind),
		Name:     info.Name,
		Exchange: info.Exchange,
		Account:  info.Account,
		Version:  info.Version,
	}
}

// ServiceToResponse маппит domain Service (из БД) — с Addr, LastSeenAt, CreatedAt.
func ServiceToResponse(svc domainsystem.Service) ServiceInfoResponse {
	r := ServiceInfoToResponse(svc.ServiceInfo)
	r.Addr = svc.Addr
	r.LastSeenAt = svc.LastSeenAt.Format(time.RFC3339)
	r.CreatedAt = svc.CreatedAt.Format(time.RFC3339)
	return r
}

func ServiceMetricsToResponse(metrics coresystem.ServiceMetrics) ServiceMetricsResponse {
	return ServiceMetricsResponse{
		Status:        ServiceStatusToString(metrics.Status),
		CpuPercent:    metrics.CpuPercent,
		MemoryPercent: metrics.MemoryPercent,
		UptimeSeconds: strconv.FormatInt(metrics.UptimeSeconds, 10),
		ReportedAt:    metrics.ReportedAt.Format(time.RFC3339),
	}
}

func ServiceStatusToString(status coresystem.ServiceStatus) string {
	switch status {
	case coresystem.ServiceStatusHealthy:
		return "ok"
	case coresystem.ServiceStatusDegraded:
		return "degraded"
	default:
		return "unknown"
	}
}
