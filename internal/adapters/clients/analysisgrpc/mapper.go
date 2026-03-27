package analysisgrpc

import (
	"fmt"

	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	commonpb "github.com/pulsoats/contracts/gen/go/common/v1"
	"github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/adapters/clients"
	"github.com/pulsoats/main/internal/domain/analysis"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapRequestToGRPC(cmd analysis.StartRunRequest) (*analysispb.StartRunRequest, error) {
	detCfg := clients.MapDetectorConfigReq(cmd.Detector)

	var fees *commonpb.Fees
	if cmd.Fees != nil {
		fees = mapFeesRequest(cmd.Fees)
	}

	return &analysispb.StartRunRequest{
		UserId:    cmd.UserID,
		Market:    clients.MapMarketSpecReq(cmd.Market),
		Interval:  cmd.Interval.String(),
		From:      timestamppb.New(cmd.From),
		To:        timestamppb.New(cmd.To),
		PriceType: string(cmd.PriceType),
		Detector:  detCfg,
		Fees:      fees,
	}, nil
}

func mapRunMetaResponse(resp *analysispb.RunMeta) (analysis.Run, error) {
	ms, ok := clients.MapMarketSpecResponse(resp.Market)
	if !ok {
		return analysis.Run{}, fmt.Errorf("map run meta response: %w: resp is nil", errorsx.ErrInvalidArgument)
	}

	interval, ok := market.ParseInterval(resp.Interval)
	if !ok {
		return analysis.Run{}, fmt.Errorf("map run meta response: %w: interval %v", errorsx.ErrNotFound, resp.Interval)
	}
	detector, ok := clients.MapDetectorConfigResponse(resp.Detector)
	if !ok {
		return analysis.Run{}, fmt.Errorf("map run meta response: %w: resp is nil", errorsx.ErrInvalidArgument)
	}

	return analysis.Run{
		ID:           resp.Id,
		Market:       ms,
		Interval:     interval,
		Detector:     detector,
		SignalsCount: resp.SignalsCount,
		AvgProfitPPM: resp.AvgProfitPpm,
		CreatedAt:    resp.CreatedAt.AsTime(),
	}, nil
}

func mapFeesRequest(fees *market.TakerMakerFees) *commonpb.Fees {
	if fees == nil {
		return nil
	}
	return &commonpb.Fees{
		TakerFee: fees.TakerFeeRate,
		MakerFee: fees.MakerFeeRate,
	}
}
