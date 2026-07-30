package analysis

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/market"
	corerun "github.com/pulsoats/core/run"
	"github.com/pulsoats/main/internal/domain/analysis"
	"github.com/pulsoats/main/internal/transport/handlers/core"
)

func newRunRequestFromRequest(req newRunRequest) (analysis.NewRunRequest, error) {
	fromTime, err := time.Parse(time.RFC3339, req.FromTime)
	if err != nil {
		return analysis.NewRunRequest{}, fmt.Errorf("fromTime: %w", errorsx.ErrInvalidArgument)
	}
	toTime, err := time.Parse(time.RFC3339, req.ToTime)
	if err != nil {
		return analysis.NewRunRequest{}, fmt.Errorf("toTime: %w", errorsx.ErrInvalidArgument)
	}

	if fromTime.IsZero() || toTime.IsZero() {
		return analysis.NewRunRequest{}, fmt.Errorf("fromTime/toTime: %w", errorsx.ErrInvalidArgument)
	}

	return analysis.NewRunRequest{
		Market: market.Spec{
			Exchange: req.Market.Exchange,
			Category: req.Market.Category,
			Symbol:   req.Market.Symbol,
		},
		Interval:         req.Interval,
		From:             fromTime,
		To:               toTime,
		DetectorConfigs:  core.DetectorConfigFromRequest(req.DetectorConfig),
		FiltersConfigs:   core.FiltersConfigsFromRequest(req.FiltersConfigs),
		Fees:             feesFromDTO(req.Fees),
		DisableStopLoss:  req.DisableStopLoss,
		DisableRepeats:   req.DisableRepeats,
		CollectRejectLog: req.CollectRejectLog,
	}, nil
}

func feesFromDTO(req *feesRequest) *market.TakerMakerFees {
	if req == nil {
		return nil
	}

	return &market.TakerMakerFees{
		TakerFeeRate: core.PercentToPPM(req.TakerFeePct),
		MakerFeeRate: core.PercentToPPM(req.MakerFeePct),
	}
}

func runToResponse(run analysis.Run) runResponse {
	var sharedAt *string
	if run.SharedAt != nil {
		sharedAt = new(run.SharedAt.Format(time.RFC3339))
	}

	return runResponse{
		BaseRunResponse: core.BaseRunToResponse(run.BaseRun),
		Fees: feesResponse{
			TakerFeePct: core.PPMToPercent(run.Fees.TakerFeeRate),
			MakerFeePct: core.PPMToPercent(run.Fees.MakerFeeRate),
		},
		SumProfitPct:    core.PPMToPercent(run.SumProfitPPM),
		AvgProfitPct:    core.PPMToPercent(run.AvgProfitPPM),
		DisableStopLoss: run.DisableStopLoss,
		DisableRepeats:  run.DisableRepeats,
		IsShared:        run.IsShared,
		SharedAt:        sharedAt,
	}
}

func runsPagedRequestFromRequest(req runsPagedRequest) (analysis.RunsPagedRequest, error) {
	var beforeID *uuid.UUID
	if req.BeforeID != "" {
		id, err := uuid.Parse(req.BeforeID)
		if err != nil {
			return analysis.RunsPagedRequest{}, fmt.Errorf("runs paged request from dto: invalid before_id: %w", errors.Join(errorsx.ErrInvalidArgument, err))
		}
		beforeID = &id
	}

	for _, s := range req.Statuses {
		if _, ok := corerun.ParseStatusCode(s); !ok {
			return analysis.RunsPagedRequest{}, fmt.Errorf("runs paged request from dto: invalid status_code: %d: %w", s, errorsx.ErrInvalidArgument)
		}
	}

	var minAvgProfitPPM, maxAvgProfitPPM *int64
	if req.MinAvgProfitPct != nil {
		minAvgProfitPPM = new(core.PercentToPPM(*req.MinAvgProfitPct))
	}
	if req.MaxAvgProfitPct != nil {
		maxAvgProfitPPM = new(core.PercentToPPM(*req.MaxAvgProfitPct))
	}

	return analysis.RunsPagedRequest{
		Limit:       req.Limit,
		BeforeID:    beforeID,
		OrderDirAsc: req.OrderDirAsc,
		Scope:       req.Scope,
		Filter: &analysis.RunsFilter{
			Exchanges:       req.Exchanges,
			Categories:      req.Categories,
			Symbols:         req.Symbols,
			Intervals:       req.Intervals,
			DetectorCodes:   req.DetectorCodes,
			Statuses:        req.Statuses,
			MinSignals:      req.MinSignals,
			MaxSignals:      req.MaxSignals,
			MinAvgProfitPPM: minAvgProfitPPM,
			MaxAvgProfitPPM: maxAvgProfitPPM,
			FirstCandleFrom: req.FirstCandleFrom,
			LastCandleTo:    req.LastCandleTo,
			CreatedFrom:     req.CreatedFrom,
			CreatedTo:       req.CreatedTo,
		},
	}, nil
}

func runsPagedResponseToResponse(resp analysis.RunsPagedResponse) runsPagedResponse {
	runs := make([]runResponse, 0, len(resp.Runs))
	for _, r := range resp.Runs {
		runs = append(runs, runToResponse(r))
	}

	return runsPagedResponse{
		Runs:         runs,
		HasMore:      resp.HasMore,
		NextBeforeID: resp.NextBeforeID,
	}
}
