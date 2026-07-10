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

func workerStatsToResponse(s live.WorkerStats) workerStatsResponse {
	return workerStatsResponse{
		RunsTotal:    s.RunsTotal,
		SignalsTotal: s.SignalsTotal,
	}
}

func runToResponse(r live.Run) runResponse {
	resp := runResponse{
		BaseRunResponse: core.BaseRunToResponse(r.Base),
		SumProfitPct:    core.PPMToPercent(r.SumProfitPPM),
	}
	if r.FinishedAt != nil {
		s := r.FinishedAt.Format(time.RFC3339)
		resp.FinishedAt = &s
	}
	if r.FinishedBy != nil {
		s := r.FinishedBy.String()
		resp.FinishedBy = &s
	}
	return resp
}

func runsPagedRequestFromDTO(req runsPagedRequest) (live.RunsPagedRequest, error) {
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

func signalToResponse(s live.EnrichedSignal) signalResponse {
	return signalResponse{
		ID:                s.ID,
		RunID:             s.RunID,
		Market:            core.MarketSpecToResponse(s.Market),
		DetectorCode:      s.DetectorCode,
		DetectorVersion:   s.DetectorVersion,
		DetectorOptsLabel: s.DetectorOptsLabel,
		CandleTime:        s.CandleTime.Unix(),
		CandleValue:       s.CandleValue,
		BuyValue:          s.BuyValue,
		TakeProfitValue:   s.TakeProfitValue,
		StopLossValue:     s.StopLossValue,
		ExpectedReturnPct: core.PPMToPercent(s.ExpectedReturnPPM),
		CreatedAt:         s.CreatedAt.Unix(),
	}
}

func signalsPagedRequestFromDTO(req signalsPagedRequest) (live.SignalsPagedRequest, error) {
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
