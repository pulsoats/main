package live

import (
	"time"

	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/main/internal/domain/live"
	domainsystem "github.com/pulsoats/main/internal/domain/system"
	"github.com/pulsoats/main/internal/transport/handlers/core"
)

func runToResponse(r live.Run) runResponse {
	resp := runResponse{
		BaseRunResponse:  core.BaseRunToResponse(r.Base),
		OrdersCount:      r.OrdersCount,
		SumProfitPercent: r.SumProfitPercent,
		FinishedBy:       r.FinishedBy,
	}
	if r.FinishedAt != nil {
		s := r.FinishedAt.Format(time.RFC3339)
		resp.FinishedAt = &s
	}
	return resp
}

func listRunsToResponse(resp live.ListRunsResponse) listRunsResponse {
	runs := make([]runResponse, 0, len(resp.Runs))
	for _, r := range resp.Runs {
		runs = append(runs, runToResponse(r))
	}
	return listRunsResponse{
		Runs:         runs,
		NextBeforeID: resp.NextBeforeID,
		HasMore:      resp.HasMore,
	}
}

func signalToResponse(s detect.Signal) signalResponse {
	return signalResponse{
		ID:                s.ID,
		RunID:             s.RunID,
		Market:            core.MarketSpecToResponse(s.Market),
		DetectorCode:      s.DetectorCode,
		DetectorOptsLabel: s.DetectorOptsLabel,
		Time:              s.Time,
		Value:             s.Value,
		BuyValue:          s.BuyValue,
		TakeProfitValue:   s.TakeProfitValue,
		StopLossValue:     s.StopLossValue,
		ExpectedReturnPPM: s.ExpectedReturnPPM,
		Fingerprint:       s.Fingerprint,
		CreatedAt:         s.CreatedAt,
	}
}

func listSignalsToResponse(resp live.ListSignalsPagedResponse) listSignalsResponse {
	signals := make([]signalResponse, 0, len(resp.Signals))
	for _, s := range resp.Signals {
		signals = append(signals, signalToResponse(s))
	}
	return listSignalsResponse{
		Signals:      signals,
		NextBeforeID: resp.NextBeforeID,
		HasMore:      resp.HasMore,
	}
}

func servicesToResponse(services []domainsystem.Service) listServicesResponse {
	result := make([]core.ServiceInfoResponse, 0, len(services))
	for _, s := range services {
		result = append(result, core.ServiceToResponse(s))
	}
	return listServicesResponse{Services: result}
}
