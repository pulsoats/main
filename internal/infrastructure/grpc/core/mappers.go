package core

import (
	"fmt"

	"github.com/google/uuid"
	catalogpb "github.com/pulsoats/contracts/gen/go/catalog/v1"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	"github.com/pulsoats/core/detect/detector"
	"github.com/pulsoats/core/detect/filter"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	"github.com/pulsoats/core/market"
	"github.com/pulsoats/core/run"
	"github.com/pulsoats/core/xgrpc"
)

func RunStatusFromProto(pb *corepb.RunStatus) (run.Status, error) {
	const op = "run status from proto"
	if pb == nil {
		return run.Status{}, fmt.Errorf("%s: message is nil: %w", op, errorsx.ErrInvalidArgument)
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

// BaseRunFromProto при ошибке вернет errorsx.ErrInternal: corepb.BaseRun создаётся внутренними сервисами,
func BaseRunFromProto(pb *corepb.BaseRun) (run.Base, error) {
	const op = "base run from proto"
	if pb == nil {
		return run.Base{}, fmt.Errorf("%s: message is nil: %w", op, errorsx.ErrInternal)
	}

	id, err := uuid.Parse(pb.Id)
	if err != nil {
		return run.Base{}, fmt.Errorf("%s: invalid run_id %q: %s: %w", op, pb.Id, err, errorsx.ErrInternal)
	}
	status, err := RunStatusFromProto(pb.Status)
	if err != nil {
		return run.Base{}, fmt.Errorf("%s: %s: %w", op, err, errorsx.ErrInternal)
	}
	marketSpec, err := xgrpc.MarketSpecFromProto(pb.Market)
	if err != nil {
		return run.Base{}, fmt.Errorf("%s: %s: %w", op, err, errorsx.ErrInternal)
	}
	interval, err := market.ParseInterval(pb.Interval)
	if err != nil {
		return run.Base{}, fmt.Errorf("%s: %s: %w", op, err, errorsx.ErrInternal)
	}
	detCfg, err := xgrpc.DetectorConfigFromProto(pb.DetectorConfig)
	if err != nil {
		return run.Base{}, fmt.Errorf("%s: %s: %w", op, err, errorsx.ErrInternal)
	}

	var filtersConfigs []filter.Config
	for _, pbFc := range pb.FiltersConfigs {
		fc, err := xgrpc.FilterConfigFromProto(pbFc)
		if err != nil {
			return run.Base{}, fmt.Errorf("%s: %s: %w", op, err, errorsx.ErrInternal)
		}
		filtersConfigs = append(filtersConfigs, fc)
	}

	return run.Base{
		ID:              id,
		Status:          status,
		Market:          marketSpec,
		Interval:        interval,
		DetectorConfig:  detCfg,
		SignalsCount:    pb.SignalsCount,
		FirstCandleTime: xgrpc.TimeFromProto(pb.FirstCandleTime),
		LastCandleTime:  xgrpc.TimeFromProto(pb.LastCandleTime),
		CreatedAt:       xgrpc.TimeFromProto(pb.CreatedAt),
		CreatedBy:       pb.CreatedBy,
	}, nil
}

func AvailableExchangesFromProto(pb *catalogpb.ListAvailableExchangesResponse) ([]exchange.Meta, error) {
	const op = "available exchanges from proto"
	if pb == nil {
		return nil, fmt.Errorf("%s: message is nil: %w", op, errorsx.ErrInternal)
	}

	metas := make([]exchange.Meta, 0, len(pb.Metas))
	for _, m := range pb.Metas {
		if m == nil {
			return nil, fmt.Errorf("%s: %w", op, errorsx.ErrInternal)
		}
		metas = append(metas, exchange.Meta{
			Code:       m.Code,
			Intervals:  m.Intervals,
			Categories: m.Categories,
		})
	}

	return metas, nil
}

func AvailableDetectorsFromProto(pb *catalogpb.ListAvailableDetectorsResponse) ([]detector.Meta, error) {
	const op = "available detectors from proto"
	if pb == nil {
		return nil, fmt.Errorf("%s: message is nil: %w", op, errorsx.ErrInternal)
	}

	metas := make([]detector.Meta, 0, len(pb.Metas))
	for _, m := range pb.Metas {
		if m == nil {
			return nil, fmt.Errorf("%s: %w", op, errorsx.ErrInternal)
		}
		metas = append(metas, detector.Meta{
			Code:        m.Code,
			Description: m.Description,
			OptsSchema:  m.OptsSchema,
			Version:     m.Version,
		})
	}

	return metas, nil
}

func AvailableFiltersFromProto(pb *catalogpb.ListAvailableFiltersResponse) ([]filter.Meta, error) {
	const op = "available filters from proto"
	if pb == nil {
		return nil, fmt.Errorf("%s: message is nil: %w", op, errorsx.ErrInternal)
	}

	metas := make([]filter.Meta, 0, len(pb.Metas))
	for _, m := range pb.Metas {
		if m == nil {
			return nil, fmt.Errorf("%s: %w", op, errorsx.ErrInternal)
		}
		metas = append(metas, filter.Meta{
			Code:        m.Code,
			Description: m.Description,
		})
	}

	return metas, nil
}
