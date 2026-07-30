package analysis

import (
	"fmt"

	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	"github.com/pulsoats/core/detect/filter"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/lib/units"
	"github.com/pulsoats/core/market"
	"github.com/pulsoats/core/xgrpc"
	"github.com/pulsoats/main/internal/domain/analysis"
	coregrpc "github.com/pulsoats/main/internal/infrastructure/grpc/core"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapNewRunRequest(req analysis.NewRunRequest) *analysispb.NewRunRequest {
	return &analysispb.NewRunRequest{
		Market:           xgrpc.MarketSpecToProto(req.Market),
		Interval:         req.Interval,
		From:             timestamppb.New(req.From),
		To:               timestamppb.New(req.To),
		DetectorConfig:   xgrpc.DetectorConfigToProto(req.DetectorConfigs),
		FiltersConfigs:   filtersConfigsToProto(req.FiltersConfigs),
		Fees:             xgrpc.FeesToProto(req.Fees),
		DisableStopLoss:  req.DisableStopLoss,
		DisableRepeats:   req.DisableRepeats,
		CollectRejectLog: req.CollectRejectLog,
	}
}

func runFromProto(pb *analysispb.Run) (analysis.Run, error) {
	const op = "run from proto"
	if pb == nil {
		return analysis.Run{}, fmt.Errorf("%s: message is nil: %w", op, errorsx.ErrInternal)
	}

	base, err := coregrpc.BaseRunFromProto(pb.BaseRun)
	if err != nil {
		return analysis.Run{}, fmt.Errorf("%s: %w", op, err)
	}

	var fees market.TakerMakerFees
	if pb.Fees != nil {
		fees = *xgrpc.FeesFromProto(pb.Fees)
	}

	return analysis.Run{
		BaseRun:         base,
		Fees:            fees,
		SumProfitPPM:    pb.SumProfitPpm,
		AvgProfitPPM:    pb.AvgProfitPpm,
		DisableStopLoss: pb.DisableStopLoss,
		DisableRepeats:  pb.DisableRepeats,
		IsShared:        pb.IsShared,
		SharedAt:        xgrpc.TimePtrFromProto(pb.SharedAt),
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
		MinAvgProfitPpm: f.MinAvgProfitPPM,
		MaxAvgProfitPpm: f.MaxAvgProfitPPM,
		DisableStopLoss: f.DisableStopLoss,
		DisableRepeats:  f.DisableRepeats,
		FirstCandleFrom: xgrpc.TimePtrToProto(f.FirstCandleFrom),
		LastCandleTo:    xgrpc.TimePtrToProto(f.LastCandleTo),
		CreatedFrom:     xgrpc.TimePtrToProto(f.CreatedFrom),
		CreatedTo:       xgrpc.TimePtrToProto(f.CreatedTo),
	}
}

func pctPtrToPpmPtr(pct *float64) *int64 {
	if pct == nil {
		return nil
	}
	return new(int64(*pct * float64(units.PPM) / 100))
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

	nextBeforeID, err := xgrpc.UUIDPtrFromProto(op, "next_before_id", pb.NextBeforeId)
	if err != nil {
		return analysis.RunsPagedResponse{}, err
	}

	return analysis.RunsPagedResponse{
		Runs:         runs,
		HasMore:      pb.HasMore,
		NextBeforeID: nextBeforeID,
	}, nil
}

func filtersConfigsToProto(cfgs []filter.Config) []*corepb.FilterConfig {
	out := make([]*corepb.FilterConfig, 0, len(cfgs))
	for _, fc := range cfgs {
		out = append(out, &corepb.FilterConfig{
			Code:   fc.Code,
			Period: int32(fc.Period),
		})
	}
	return out
}
