package core

import (
	"errors"
	"time"

	"github.com/google/uuid"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/exchange"
	"github.com/pulsoats/core/market"
	"github.com/pulsoats/core/run"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func MarketSpecToProto(spec market.Spec) *corepb.MarketSpec {
	return &corepb.MarketSpec{
		Exchange: spec.Exchange,
		Category: spec.Category,
		Symbol:   spec.Symbol,
	}
}

func MarketSpecFromProto(pb *corepb.MarketSpec) (market.Spec, bool) {
	if pb == nil {
		return market.Spec{}, false
	}
	return market.Spec{
		Exchange: pb.Exchange,
		Category: pb.Category,
		Symbol:   pb.Symbol,
	}, true
}

func DetectorConfigToProto(cfg detect.DetectorConfig) *corepb.DetectorConfig {
	return &corepb.DetectorConfig{
		Code:      cfg.Code,
		OptsLabel: cfg.OptsLabel,
		Opts:      cfg.Opts,
	}
}

func DetectorConfigFromProto(pb *corepb.DetectorConfig) (detect.DetectorConfig, bool) {
	if pb == nil {
		return detect.DetectorConfig{}, false
	}
	return detect.DetectorConfig{
		Code:      pb.Code,
		OptsLabel: pb.OptsLabel,
		Opts:      pb.Opts,
	}, true
}

func DetectorMetaFromProto(pb *corepb.DetectorMeta) (detect.DetectorMeta, bool) {
	if pb == nil {
		return detect.DetectorMeta{}, false
	}
	return detect.DetectorMeta{
		Code:        pb.Code,
		Description: pb.Description,
		Kind:        detect.DetectorKind(pb.Kind),
		OptsSchema:  pb.OptsSchema,
		Version:     pb.Version,
	}, true
}

func FeesToProto(fees *market.TakerMakerFees) *corepb.Fees {
	if fees == nil {
		return nil
	}
	return &corepb.Fees{
		TakerFee: fees.TakerFeeRate,
		MakerFee: fees.MakerFeeRate,
	}
}

func FeesFromProto(fees *corepb.Fees) (market.TakerMakerFees, bool) {
	if fees == nil {
		return market.TakerMakerFees{}, false
	}
	return market.TakerMakerFees{
		TakerFeeRate: fees.TakerFee,
		MakerFeeRate: fees.MakerFee,
	}, true
}

func RunStatusFromProto(pb *corepb.RunStatus) (run.Status, error) {
	if pb == nil {
		return run.Status{}, errors.New("run status: resp is nil")
	}

	code, ok := run.ParseStatusCode(int(pb.Code))
	if !ok {
		return run.Status{}, errors.New("unexpected run status code")
	}

	return run.Status{
		Code:    code,
		Message: pb.Message,
	}, nil
}

func PbTimeToRFC3339(timestamp *timestamppb.Timestamp) (string, bool) {
	if timestamp == nil {
		return "", false
	}

	return timestamp.AsTime().Format(time.RFC3339), true
}

func BaseRunFromProto(pb *corepb.BaseRun) (run.Base, error) {
	if pb == nil {
		return run.Base{}, errors.New("nil pb")
	}

	id, err := uuid.Parse(pb.Id)
	if err != nil {
		return run.Base{}, errors.New("runId")
	}
	status, err := RunStatusFromProto(pb.Status)
	if err != nil {
		return run.Base{}, err
	}
	marketSpec, ok := MarketSpecFromProto(pb.Market)
	if !ok {
		return run.Base{}, errors.New("market: nil pb %w")
	}
	interval, ok := market.ParseInterval(pb.Interval)
	if !ok {
		return run.Base{}, errors.New("interval: unexpected value")
	}
	detector, ok := DetectorConfigFromProto(pb.Detector)
	if !ok {
		return run.Base{}, errors.New("detector: nil pb %w")
	}

	return run.Base{
		ID:              id,
		Status:          status,
		Market:          marketSpec,
		Interval:        interval,
		Detector:        detector,
		SignalsCount:    pb.GetSignalsCount(),
		FirstCandleTime: pb.FirstCandleTime.AsTime(),
		LastCandleTime:  pb.LastCandleTime.AsTime(),
		CreatedAt:       pb.CreatedAt.AsTime(),
		CreatedBy:       pb.CreatedBy,
	}, nil
}

func ExchangeMetaFromProto(pb *corepb.ExchangeMeta) (exchange.Meta, error) {
	if pb == nil {
		return exchange.Meta{}, errors.New("resp is nil")
	}

	if pb.Intervals == nil {
		return exchange.Meta{}, errors.New("intervals is nil")
	}

	if pb.Categories == nil {
		return exchange.Meta{}, errors.New("categories is nil")
	}

	return exchange.Meta{
		Code:       pb.Code,
		Intervals:  pb.Intervals,
		Categories: pb.Categories,
	}, nil
}
