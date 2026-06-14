package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	catalogpb "github.com/pulsoats/contracts/gen/go/catalog/v1"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
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
		Version:   cfg.Version,
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
		Version:   pb.Version,
		OptsLabel: pb.OptsLabel,
		Opts:      pb.Opts,
	}, true
}

func DetectorMetaFromProto(pb *corepb.DetectorMeta) (detect.DetectorMeta, error) {
	if pb == nil {
		return detect.DetectorMeta{}, fmt.Errorf("detector meta from proto: resp is nil: %w", errorsx.ErrInternal)
	}
	return detect.DetectorMeta{
		Code:        pb.Code,
		Description: pb.Description,
		Kind:        detect.DetectorKind(pb.Kind),
		OptsSchema:  pb.OptsSchema,
		Version:     pb.Version,
	}, nil
}

func FeesToProto(fees *market.TakerMakerFees) *corepb.Fees {
	if fees == nil {
		return nil
	}
	return &corepb.Fees{
		TakerFeePpm: fees.TakerFeeRate,
		MakerFeePpm: fees.MakerFeeRate,
	}
}

func FeesFromProto(fees *corepb.Fees) (market.TakerMakerFees, bool) {
	if fees == nil {
		return market.TakerMakerFees{}, false
	}
	return market.TakerMakerFees{
		TakerFeeRate: fees.TakerFeePpm,
		MakerFeeRate: fees.MakerFeePpm,
	}, true
}

func RunStatusFromProto(pb *corepb.RunStatus) (run.Status, error) {
	const op = "run status from proto"
	if pb == nil {
		return run.Status{}, fmt.Errorf("%s: pb is nil: %w", op, errorsx.ErrInternal)
	}

	code, ok := run.ParseStatusCode(int(pb.Code))
	if !ok {
		return run.Status{}, fmt.Errorf("%s: unexpected run status %d: %w", op, pb.Code, errorsx.ErrInternal)
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
	const op = "base run from proto"
	if pb == nil {
		return run.Base{}, fmt.Errorf("%s: nil pb: %w", op, errorsx.ErrInternal)
	}

	id, err := uuid.Parse(pb.Id)
	if err != nil {
		return run.Base{}, fmt.Errorf("%s: invalid run_id %q: %w", op, pb.Id, errors.Join(errorsx.ErrInternal, err))
	}
	status, err := RunStatusFromProto(pb.Status)
	if err != nil {
		return run.Base{}, fmt.Errorf("%s: %w", op, err)
	}
	marketSpec, ok := MarketSpecFromProto(pb.Market)
	if !ok {
		return run.Base{}, fmt.Errorf("%s: nil market spec: %w", op, errorsx.ErrInternal)
	}
	interval, ok := market.ParseInterval(pb.Interval)
	if !ok {
		return run.Base{}, fmt.Errorf("%s: unexpected interval %q: %w", op, pb.Interval, errorsx.ErrInternal)
	}
	detector, ok := DetectorConfigFromProto(pb.Detector)
	if !ok {
		return run.Base{}, fmt.Errorf("%s: detector_config pb is nil: %w", op, errorsx.ErrInternal)
	}
	// created_at — обязательное поле (в pb не optional): nil = непредвиденно.
	createdAt, err := RequireTime(op, "created_at", pb.CreatedAt)
	if err != nil {
		return run.Base{}, err
	}

	return run.Base{
		ID:           id,
		Status:       status,
		Market:       marketSpec,
		Interval:     interval,
		Detector:     detector,
		SignalsCount: pb.SignalsCount,
		// first/last_candle_time — optional: отсутствие легально (прогон без свечей) → zero time.
		FirstCandleTime: TimeFromProto(pb.FirstCandleTime),
		LastCandleTime:  TimeFromProto(pb.LastCandleTime),
		CreatedAt:       createdAt,
		CreatedBy:       pb.CreatedBy,
	}, nil
}

func ExchangeMetaFromProto(pb *corepb.ExchangeMeta) (exchange.Meta, error) {
	const op = "exchange meta from proto"
	if pb == nil {
		return exchange.Meta{}, fmt.Errorf("%s: pb is nil: %w", op, errorsx.ErrInternal)
	}

	return exchange.Meta{
		Code:       pb.Code,
		Intervals:  pb.Intervals,
		Categories: pb.Categories,
	}, nil
}

func AvailableExchangesFromProto(pb *catalogpb.ListAvailableExchangesResponse) ([]exchange.Meta, error) {
	const op = "available exchanges from proto"
	if pb == nil {
		return nil, fmt.Errorf("%s: pb is nil: %w", op, errorsx.ErrInternal)
	}

	metas := make([]exchange.Meta, 0, len(pb.ExchangeMetas))
	for _, m := range pb.ExchangeMetas {
		mFromProto, err := ExchangeMetaFromProto(m)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		metas = append(metas, mFromProto)
	}

	return metas, nil
}

func AvailableDetectorsFromProto(pb *catalogpb.ListAvailableDetectorsResponse) ([]detect.DetectorMeta, error) {
	const op = "available detectors from proto"
	if pb == nil {
		return nil, fmt.Errorf("%s: pb is nil: %w", op, errorsx.ErrInternal)
	}

	metas := make([]detect.DetectorMeta, 0, len(pb.Detectors))
	for _, det := range pb.Detectors {
		mFromProto, err := DetectorMetaFromProto(det)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		metas = append(metas, mFromProto)
	}

	return metas, nil
}
