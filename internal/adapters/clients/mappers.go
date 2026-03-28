package clients

import (
	commonpb "github.com/pulsoats/contracts/gen/go/common/v1"
	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/market"
)

func MapMarketSpecReq(spec market.Spec) *commonpb.MarketSpec {
	return &commonpb.MarketSpec{
		Exchange: spec.Exchange,
		Category: string(spec.Category),
		Symbol:   spec.Symbol,
	}
}

func MapDetectorConfigReq(cfg detect.DetectorConfig) *commonpb.DetectorConfig {
	return &commonpb.DetectorConfig{
		Code:  cfg.Code,
		Label: cfg.Label,
		Opts:  cfg.Opts,
	}
}

func MapDetectorConfigResponse(resp *commonpb.DetectorConfig) (detect.DetectorConfig, bool) {
	if resp == nil {
		return detect.DetectorConfig{}, false
	}
	return detect.DetectorConfig{
		Code:  resp.Code,
		Label: resp.Label,
		Opts:  resp.Opts,
	}, true
}

func MapFeesResponse(fees *commonpb.Fees) (market.TakerMakerFees, bool) {
	if fees == nil {
		return market.TakerMakerFees{}, false
	}
	return market.TakerMakerFees{
		TakerFeeRate: fees.TakerFee,
		MakerFeeRate: fees.MakerFee,
	}, true
}

func MapMarketSpecResponse(resp *commonpb.MarketSpec) (market.Spec, bool) {
	if resp == nil {
		return market.Spec{}, false
	}
	return market.Spec{
		Exchange: resp.Exchange,
		Category: market.Category(resp.Category),
		Symbol:   resp.Symbol,
	}, true
}
