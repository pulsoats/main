package live

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
	corerun "github.com/pulsoats/core/run"
	"github.com/pulsoats/main/internal/domain/live"
	"github.com/pulsoats/main/internal/transport/handlers/core"
)

func accountToResponse(a live.ExchangeAccount) exchangeAccountResponse {
	return exchangeAccountResponse{
		ID:        a.ID,
		Exchange:  a.Exchange,
		Name:      a.Name,
		Email:     a.Email,
		ExpiresAt: a.ExpiresAt.Format(time.RFC3339),
		CreatedAt: a.CreatedAt.Format(time.RFC3339),
		UpdatedAt: a.UpdatedAt.Format(time.RFC3339),
	}
}

func accountsToResponse(accounts []live.ExchangeAccount) exchangeAccountsResponse {
	res := make([]exchangeAccountResponse, 0, len(accounts))
	for _, a := range accounts {
		res = append(res, accountToResponse(a))
	}
	return exchangeAccountsResponse{Accounts: res}
}

func nodeToResponse(n live.Node) nodeResponse {
	return nodeResponse{
		ID:           n.ID,
		Name:         n.Name,
		Exchange:     n.Exchange,
		Host:         n.Host,
		DockerPort:   n.DockerPort,
		Region:       n.Region,
		MaxWorkers:   n.MaxWorkers,
		WorkersCount: n.WorkersCount,
		Status:       string(n.Status),
		LastError:    n.LastError,
		CreatedAt:    n.CreatedAt.Format(time.RFC3339),
	}
}

func nodesToResponse(nodes []live.Node) nodesResponse {
	res := make([]nodeResponse, 0, len(nodes))
	for _, n := range nodes {
		res = append(res, nodeToResponse(n))
	}
	return nodesResponse{Nodes: res}
}

func workerToResponse(w live.Worker) workerResponse {
	return workerResponse{
		ID:                w.ID,
		NodeID:            w.NodeID,
		Host:              w.Host,
		GRPCPort:          w.GRPCPort,
		ContainerID:       w.ContainerID,
		ExchangeAccountID: w.ExchangeAccountID,
		Status:            string(w.Status),
		LastError:         w.LastError,
		CreatedAt:         w.CreatedAt.Format(time.RFC3339),
	}
}

func workersToResponse(workers []live.Worker) workersResponse {
	res := make([]workerResponse, 0, len(workers))
	for _, w := range workers {
		res = append(res, workerToResponse(w))
	}
	return workersResponse{Workers: res}
}

func workerMetricsToResponse(s live.WorkerMetrics) workerMetricsResponse {
	var workerStatsResp *workerStatsResponse
	if s.WorkerStats != nil {
		workerStatsResp = new(workerStatsResponse{
			ActiveRuns:   s.WorkerStats.ActiveRuns,
			SignalsTotal: s.WorkerStats.SignalsTotal,
		})
	}

	var resourceUsageResp *resourceUsageResponse
	if s.ResourceUsage != nil {
		resourceUsageResp = new(resourceUsageResponse{
			ContainerID: s.ResourceUsage.ContainerID,
			CPUPercent:  s.ResourceUsage.CPUPercent,
			MemoryBytes: s.ResourceUsage.MemoryBytes,
		})
	}
	return workerMetricsResponse{
		WorkerID:      s.WorkerID,
		Status:        string(s.Status),
		WorkerStats:   workerStatsResp,
		ResourceUsage: resourceUsageResp,
		At:            s.At.Format(time.RFC3339),
	}
}

func newRunFromRequest(req newRunRequest) live.NewRunRequest {
	return live.NewRunRequest{
		MarketSpec:     core.MarketSpecFromRequest(req.Market),
		Interval:       req.Interval,
		DetectorConfig: core.DetectorConfigFromRequest(req.DetectorConfig),
		FiltersConfigs: core.FiltersConfigsFromRequest(req.FiltersConfigs),
	}
}

func runToResponse(r live.Run) runResponse {
	resp := runResponse{
		BaseRunResponse: core.BaseRunToResponse(r.Base),
	}
	if r.FinishedAt != nil {
		resp.FinishedAt = new(r.FinishedAt.Format(time.RFC3339))
	}
	if r.FinishedBy != nil {
		resp.FinishedBy = new(r.FinishedBy.String())
	}
	return resp
}

func runsPagedFromRequest(req runsPagedRequest) (live.RunsPagedRequest, error) {
	var beforeID *uuid.UUID
	if req.BeforeID != "" {
		id, err := uuid.Parse(req.BeforeID)
		if err != nil {
			return live.RunsPagedRequest{}, fmt.Errorf("runs paged request from dto: invalid before_id: %w", errors.Join(errorsx.ErrInvalidArgument, err))
		}
		beforeID = &id
	}

	for _, s := range req.Statuses {
		if _, ok := corerun.ParseStatusCode(s); !ok {
			return live.RunsPagedRequest{}, fmt.Errorf("runs paged request from dto: invalid status_code: %d: %w", s, errorsx.ErrInvalidArgument)
		}
	}

	return live.RunsPagedRequest{
		Limit:       req.Limit,
		BeforeID:    beforeID,
		OrderDirAsc: req.OrderDirAsc,
		Filter: &live.RunsFilter{
			Categories:    req.Categories,
			Symbols:       req.Symbols,
			Intervals:     req.Intervals,
			DetectorCodes: req.DetectorCodes,
			Statuses:      req.Statuses,
			MinSignals:    req.MinSignals,
			MaxSignals:    req.MaxSignals,
			CreatedFrom:   req.CreatedFrom,
			CreatedTo:     req.CreatedTo,
		},
	}, nil
}

func runsPagedToResponse(resp live.RunsPagedResponse) runsPagedResponse {
	runs := make([]runResponse, 0, len(resp.Runs))
	for _, r := range resp.Runs {
		runs = append(runs, runToResponse(r))
	}
	return runsPagedResponse{
		Runs:         runs,
		NextBeforeID: resp.NextBeforeID,
		HasMore:      resp.HasMore,
	}
}

func runStatusEventToResponse(event live.RunStatusEvent) runStatusEventResponse {
	return runStatusEventResponse{
		RunID:  event.RunID,
		Status: core.RunStatusToResponse(event.Status),
	}
}

func signalToResponse(s live.EnrichedSignal) signalResponse {
	return signalResponse{
		ID:                s.ID,
		RunID:             s.RunID,
		Market:            core.MarketSpecToResponse(s.Market),
		Interval:          s.Interval,
		DetectorCode:      s.DetectorCode,
		DetectorVersion:   s.DetectorVersion,
		CandleTime:        s.CandleTime.Format(time.RFC3339),
		BuyValue:          s.BuyValue,
		TakeProfitValue:   s.TakeProfitValue,
		StopLossValue:     s.StopLossValue,
		ExpectedReturnPct: core.PPMToPercent(s.ExpectedReturnPPM),
		CreatedAt:         s.CreatedAt.Format(time.RFC3339),
	}
}

func signalsPagedFromRequest(req signalsPagedRequest) (live.SignalsPagedRequest, error) {
	const op = "signals paged request from dto"

	if req.Limit <= 0 {
		return live.SignalsPagedRequest{}, fmt.Errorf("%s: invalid limit: %w", op, errorsx.ErrInvalidArgument)
	}
	var beforeID *uuid.UUID
	if req.BeforeID != "" {
		id, err := uuid.Parse(req.BeforeID)
		if err != nil {
			return live.SignalsPagedRequest{}, fmt.Errorf("%s: invalid before_id: %w", op, errors.Join(errorsx.ErrInvalidArgument, err))
		}
		beforeID = &id
	}

	var runID *uuid.UUID
	if req.RunID != "" {
		id, err := uuid.Parse(req.RunID)
		if err != nil {
			return live.SignalsPagedRequest{}, fmt.Errorf("%s: invalid run_id: %w", op, errors.Join(errorsx.ErrInvalidArgument, err))
		}
		runID = &id
	}

	return live.SignalsPagedRequest{
		Limit:       req.Limit,
		BeforeID:    beforeID,
		OrderDirAsc: req.OrderDirAsc,
		Filter: &live.SignalsFilter{
			RunID:         runID,
			Categories:    req.Categories,
			Symbols:       req.Symbols,
			Intervals:     req.Intervals,
			DetectorCodes: req.DetectorCodes,
			CreatedFrom:   req.CreatedFrom,
			CreatedTo:     req.CreatedTo,
		},
	}, nil
}

func signalsPagedToResponse(resp live.SignalsPagedResponse) signalsPagedResponse {
	signals := make([]signalResponse, 0, len(resp.Signals))
	for _, s := range resp.Signals {
		signals = append(signals, signalToResponse(s))
	}
	return signalsPagedResponse{
		Signals:      signals,
		NextBeforeID: resp.NextBeforeID,
		HasMore:      resp.HasMore,
	}
}
