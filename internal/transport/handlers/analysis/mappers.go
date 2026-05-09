package analysis

import (
	"fmt"
	"time"

	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/lib/units"
	"github.com/pulsoats/core/market"
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
		Interval: req.Interval,
		From:     fromTime,
		To:       toTime,
		Detector: detect.DetectorConfig{
			Code:      req.Detector.Code,
			OptsLabel: req.Detector.OptsLabel,
			Opts:      req.Detector.Opts,
		},
		Fees: feesFromDTO(req.Fees),
	}, nil
}

func feesFromDTO(req *feesRequest) *market.TakerMakerFees {
	if req == nil {
		return nil
	}

	return &market.TakerMakerFees{
		TakerFeeRate: int64(req.TakerFee * float64(units.PPM)),
		MakerFeeRate: int64(req.MakerFee * float64(units.PPM)),
	}
}

func runToResponse(run analysis.Run) runResponse {
	var sharedAt *string
	if run.SharedAt != nil {
		sharedAtStr := run.SharedAt.Format(time.RFC3339)
		sharedAt = &sharedAtStr
	}

	return runResponse{
		BaseRunResponse:  core.BaseRunToResponse(run.BaseRun),
		AvgProfitPercent: run.AvgProfitPercent,
		IsShared:         run.IsShared,
		SharedAt:         sharedAt,
	}
}

func listRunsResponseToResponse(resp analysis.ListRunsPagedResponse) listRunsResponse {
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
