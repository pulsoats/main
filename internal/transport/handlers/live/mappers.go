package live

import (
	"time"

	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/main/internal/domain/live"
	"github.com/pulsoats/main/internal/transport/handlers/core"
)

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

func nodeToResponse(n live.Node) nodeResponse {
	return nodeResponse{
		ID:           n.ID,
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

func listRunsToResponse(resp live.RunsResponse) listRunsResponse {
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

func listSignalsToResponse(resp live.SignalsPagedResponse) listSignalsResponse {
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
