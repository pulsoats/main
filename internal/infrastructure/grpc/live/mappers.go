package live

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	livepb "github.com/pulsoats/contracts/gen/go/live/v1"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/lib/units"
	"github.com/pulsoats/core/market"
	"github.com/pulsoats/main/internal/domain/live"
	"github.com/pulsoats/main/internal/infrastructure/grpc/core"
)

func runFromProto(pb *livepb.Run) (live.Run, error) {
	const op = "run from proto"
	if pb == nil {
		return live.Run{}, fmt.Errorf("%s: nil pb: %w", op, errorsx.ErrInternal)
	}

	baseRun, err := core.BaseRunFromProto(pb.BaseRun)
	if err != nil {
		return live.Run{}, err
	}

	var finishedAt *time.Time
	if pb.FinishedAt != nil {
		t := pb.FinishedAt.AsTime()
		finishedAt = &t
	}

	var finishedBy *uuid.UUID
	if pb.FinishedBy != nil {
		id, err := uuid.Parse(*pb.FinishedBy)
		if err != nil {
			return live.Run{}, fmt.Errorf("%s: invalid finished_by id: %w", op, errorsx.ErrInternal)
		}
		finishedBy = &id
	}
	return live.Run{
		Base:             baseRun,
		SumProfitPercent: float64(pb.SumProfitPpm) / float64(units.PPM) * 100,
		FinishedAt:       finishedAt,
		FinishedBy:       finishedBy,
	}, nil
}

func signalFromProto(pb *livepb.Signal) (detect.Signal, error) {
	const op = "signal from proto"
	if pb == nil {
		return detect.Signal{}, fmt.Errorf("%s: pb is nil: %w", op, errorsx.ErrInternal)
	}

	id, err := uuid.Parse(pb.Id)
	if err != nil {
		return detect.Signal{}, fmt.Errorf("%s: invalid signal_id: %w", op, errorsx.ErrInternal)
	}

	runID, err := uuid.Parse(pb.RunId)
	if err != nil {
		return detect.Signal{}, fmt.Errorf("%s: invalid run_id: %w", op, errorsx.ErrInternal)
	}

	marketSpec, ok := core.MarketSpecFromProto(pb.Market)
	if !ok {
		return detect.Signal{}, fmt.Errorf("%s: marketSpec is nil: %w", op, errorsx.ErrInternal)
	}

	interval, ok := market.ParseInterval(pb.Interval)
	if !ok {
		return detect.Signal{}, fmt.Errorf("%s: unexpected interval: %w", op, errorsx.ErrInternal)
	}

	if pb.CandleTime == nil {
		return detect.Signal{}, fmt.Errorf("%s: candle_time is nil: %w", op, errorsx.ErrInternal)
	}

	if pb.CreatedAt == nil {
		return detect.Signal{}, fmt.Errorf("%s: created_at is nil: %w", op, errorsx.ErrInternal)
	}

	return detect.Signal{
		ID:                id,
		RunID:             runID,
		Market:            marketSpec,
		Interval:          interval,
		DetectorCode:      pb.DetectorCode,
		DetectorVersion:   pb.DetectorVersion,
		DetectorOptsLabel: pb.DetectorOptsLabel,
		CandleTime:        pb.CandleTime.AsTime().UnixMilli(),
		CandleValue:       pb.CandleValue,
		BuyValue:          pb.BuyValue,
		TakeProfitValue:   pb.TakeProfitValue,
		StopLossValue:     pb.StopLossValue,
		ExpectedReturnPPM: pb.ExpectedReturnPpm,
		CreatedAt:         pb.CreatedAt.AsTime().UnixMilli(),
	}, nil
}

func eventFromProto(pb *livepb.Event) (live.Event, error) {
	const op = "event from proto"
	if pb == nil {
		return live.Event{}, fmt.Errorf("%s: pb is nil: %w", op, errorsx.ErrInternal)
	}

	runID, err := uuid.Parse(pb.RunId)
	if err != nil {
		return live.Event{}, fmt.Errorf("%s: invalid run_id: %w", op, errorsx.ErrInternal)
	}
	switch p := pb.Payload.(type) {
	case *livepb.Event_Run:
		r, err := runFromProto(p.Run)
		if err != nil {
			return live.Event{}, fmt.Errorf("%s: %w", op, err)
		}
		return live.Event{
			RunID:   runID,
			Payload: live.RunEvent{Run: r},
		}, nil
	case *livepb.Event_Signal:
		s, err := signalFromProto(p.Signal)
		if err != nil {
			return live.Event{}, fmt.Errorf("%s: %w", op, err)
		}
		return live.Event{
			RunID:   runID,
			Payload: live.SignalEvent{Signal: s},
		}, nil
	default:
		return live.Event{}, fmt.Errorf("%s: unknown event type: %w", op, errorsx.ErrInternal)
	}
}

func runsFilterToProto(f *live.RunsFilter) *livepb.ListRunsFilter {
	if f == nil {
		return nil
	}

	statuses := make([]int32, 0, len(f.Statuses))
	for _, s := range f.Statuses {
		statuses = append(statuses, int32(s))
	}

	return &livepb.ListRunsFilter{
		Categories:    f.Categories,
		Symbols:       f.Symbols,
		Intervals:     f.Intervals,
		DetectorCodes: f.DetectorCodes,
		Statuses:      statuses,
		MinSignals:    f.MinSignals,
		MaxSignals:    f.MaxSignals,
		CreatedFrom:   core.TimePtrToProto(f.CreatedFrom),
		CreatedTo:     core.TimePtrToProto(f.CreatedTo),
	}
}

func runsPagedRequestToProto(req live.RunsPagedRequest) *livepb.ListRunsPagedRequest {
	return &livepb.ListRunsPagedRequest{
		Limit: req.Limit,
		BeforeId: func() *string {
			if req.BeforeID != nil {
				s := req.BeforeID.String()
				return &s
			}
			return nil
		}(),
		OrderDirAsc: req.OrderDirAsc,
		Filter:      runsFilterToProto(req.Filter),
	}
}

func runsPagedResponseFromProto(pb *livepb.ListRunsPagedResponse) (live.RunsPagedResponse, error) {
	const op = "runs paged response from proto"
	if pb == nil {
		return live.RunsPagedResponse{}, fmt.Errorf("%s: pb is nil: %w", op, errorsx.ErrInternal)
	}

	runs := make([]live.Run, 0, len(pb.Runs))
	for _, runPb := range pb.Runs {
		run, err := runFromProto(runPb)
		if err != nil {
			return live.RunsPagedResponse{}, fmt.Errorf("%s: %w", op, err)
		}
		runs = append(runs, run)
	}

	var nextBeforeID *uuid.UUID
	if pb.NextBeforeId != nil {
		id, err := uuid.Parse(pb.GetNextBeforeId())
		if err != nil {
			return live.RunsPagedResponse{}, fmt.Errorf("%s: invalid next_before_id: %w", op, errorsx.ErrInternal)
		}
		nextBeforeID = &id
	}

	return live.RunsPagedResponse{
		Runs:         runs,
		HasMore:      pb.HasMore,
		NextBeforeID: nextBeforeID,
	}, nil
}

func signalsFilterToProto(f *live.SignalsFilter) *livepb.ListSignalsFilter {
	if f == nil {
		return nil
	}

	var runIDStr *string
	if f.RunID != nil {
		s := f.RunID.String()
		runIDStr = &s
	}

	return &livepb.ListSignalsFilter{
		RunId:         runIDStr,
		Categories:    f.Categories,
		Symbols:       f.Symbols,
		Intervals:     f.Intervals,
		DetectorCodes: f.DetectorCodes,
		CreatedFrom:   core.TimePtrToProto(f.CreatedFrom),
		CreatedTo:     core.TimePtrToProto(f.CreatedTo),
	}
}

func signalsPagedRequestToProto(req live.SignalsPagedRequest) *livepb.ListSignalsPagedRequest {
	return &livepb.ListSignalsPagedRequest{
		Limit: req.Limit,
		BeforeId: func() *string {
			if req.BeforeID != nil {
				s := req.BeforeID.String()
				return &s
			}
			return nil
		}(),
		Filter: signalsFilterToProto(req.Filter),
	}
}

func signalsPagedResponseFromProto(pb *livepb.ListSignalsPagedResponse) (live.SignalsPagedResponse, error) {
	const op = "signals paged response from proto"
	if pb == nil {
		return live.SignalsPagedResponse{}, fmt.Errorf("%s: pb is nil: %w", op, errorsx.ErrInternal)
	}

	signals := make([]detect.Signal, 0, len(pb.Signals))
	for _, sigPb := range pb.Signals {
		sig, err := signalFromProto(sigPb)
		if err != nil {
			return live.SignalsPagedResponse{}, fmt.Errorf("%s: %w", op, err)
		}
		signals = append(signals, sig)
	}

	var nextBeforeID *uuid.UUID
	if pb.NextBeforeId != nil {
		id, err := uuid.Parse(pb.GetNextBeforeId())
		if err != nil {
			return live.SignalsPagedResponse{}, fmt.Errorf("%s: invalid next_before_id: %w", op, errorsx.ErrInternal)
		}
		nextBeforeID = &id
	}

	return live.SignalsPagedResponse{
		Signals:      signals,
		HasMore:      pb.HasMore,
		NextBeforeID: nextBeforeID,
	}, nil
}
