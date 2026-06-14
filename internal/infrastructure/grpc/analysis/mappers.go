package analysis

import (
	"fmt"

	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/lib/units"
	"github.com/pulsoats/main/internal/domain/analysis"
	"github.com/pulsoats/main/internal/infrastructure/grpc/core"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapNewRunRequest(req analysis.NewRunRequest) *analysispb.NewRunRequest {
	return &analysispb.NewRunRequest{
		Market:         core.MarketSpecToProto(req.Market),
		Interval:       req.Interval,
		From:           timestamppb.New(req.From),
		To:             timestamppb.New(req.To),
		DetectorConfig: core.DetectorConfigToProto(req.Detector),
		Fees:           core.FeesToProto(req.Fees),
	}
}

func runFromProto(pb *analysispb.Run) (analysis.Run, error) {
	const op = "run from proto"
	if pb == nil {
		return analysis.Run{}, fmt.Errorf("%s: pb is nil: %w", op, errorsx.ErrInternal)
	}

	base, err := core.BaseRunFromProto(pb.BaseRun)
	if err != nil {
		return analysis.Run{}, fmt.Errorf("%s: %w", op, err)
	}

	fees, _ := core.FeesFromProto(pb.Fees)

	return analysis.Run{
		BaseRun:          base,
		Fees:             fees,
		SumProfitPercent: float64(pb.SumProfitPpm) / float64(units.PPM) * 100,
		AvgProfitPercent: float64(pb.AvgProfitPpm) / float64(units.PPM) * 100,
		IsShared:         pb.IsShared,
		SharedAt:         core.TimePtrFromProto(pb.GetSharedAt()),
	}, nil
}

func runsFilterToProto(f *analysis.RunsFilter) *analysispb.ListRunsFilter {
	if f == nil {
		return nil
	}

	statuses := make([]int32, 0, len(f.Statuses))
	for _, s := range f.Statuses {
		statuses = append(statuses, int32(s))
	}

	return &analysispb.ListRunsFilter{
		Exchanges:       f.Exchanges,
		Categories:      f.Categories,
		Symbols:         f.Symbols,
		Intervals:       f.Intervals,
		DetectorCodes:   f.DetectorCodes,
		Statuses:        statuses,
		MinSignals:      f.MinSignals,
		MaxSignals:      f.MaxSignals,
		MinAvgProfitPpm: pctPtrToPpmPtr(f.MinAvgProfitPct),
		MaxAvgProfitPpm: pctPtrToPpmPtr(f.MaxAvgProfitPct),
		FirstCandleFrom: core.TimePtrToProto(f.FirstCandleFrom),
		LastCandleTo:    core.TimePtrToProto(f.LastCandleTo),
		CreatedFrom:     core.TimePtrToProto(f.CreatedFrom),
		CreatedTo:       core.TimePtrToProto(f.CreatedTo),
	}
}

func pctPtrToPpmPtr(pct *float64) *int64 {
	if pct == nil {
		return nil
	}
	v := int64(*pct * float64(units.PPM) / 100)
	return &v
}

func runsPagedResponseFromProto(pb *analysispb.ListRunsPagedResponse) (analysis.RunsPagedResponse, error) {
	const op = "runs paged from proto"
	if pb == nil {
		return analysis.RunsPagedResponse{}, fmt.Errorf("%s: resp is nil: %w", op, errorsx.ErrInternal)
	}

	runs := make([]analysis.Run, 0, len(pb.Runs))
	for _, r := range pb.Runs {
		mapped, err := runFromProto(r)
		if err != nil {
			return analysis.RunsPagedResponse{}, fmt.Errorf("%s: %w", op, err)
		}
		runs = append(runs, mapped)
	}

	nextBeforeID, err := core.UUIDPtrFromProto(op, "next_before_id", pb.NextBeforeId)
	if err != nil {
		return analysis.RunsPagedResponse{}, err
	}

	return analysis.RunsPagedResponse{
		Runs:         runs,
		HasMore:      pb.HasMore,
		NextBeforeID: nextBeforeID,
	}, nil
}
