package live

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	livepb "github.com/pulsoats/contracts/gen/go/live/v1"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/xgrpc"
	"github.com/pulsoats/main/internal/domain/live"
	"github.com/pulsoats/main/internal/infrastructure/grpc/core"
)

func runFromProto(pb *livepb.Run) (live.Run, error) {
	const op = "run from proto"
	if pb == nil {
		return live.Run{}, fmt.Errorf("%s: message is nil: %w", op, errorsx.ErrInternal)
	}

	baseRun, err := core.BaseRunFromProto(pb.BaseRun)
	if err != nil {
		return live.Run{}, err
	}

	finishedBy, err := xgrpc.UUIDPtrFromProto(op, "finished_by", pb.FinishedBy)
	if err != nil {
		return live.Run{}, err
	}

	return live.Run{
		Base:       baseRun,
		FinishedAt: xgrpc.TimePtrFromProto(pb.FinishedAt),
		FinishedBy: finishedBy,
	}, nil
}

func signalFromProto(pb *livepb.Signal) (live.EnrichedSignal, error) {
	const op = "signal from proto"
	if pb == nil {
		return live.EnrichedSignal{}, fmt.Errorf("%s: message is nil: %w", op, errorsx.ErrInternal)
	}

	id, err := uuid.Parse(pb.Id)
	if err != nil {
		return live.EnrichedSignal{}, fmt.Errorf("%s: invalid signal_id: %w", op, errorsx.ErrInternal)
	}

	runID, err := uuid.Parse(pb.RunId)
	if err != nil {
		return live.EnrichedSignal{}, fmt.Errorf("%s: invalid run_id: %w", op, errorsx.ErrInternal)
	}

	marketSpec, err := xgrpc.MarketSpecFromProto(pb.Market)
	if err != nil {
		return live.EnrichedSignal{}, fmt.Errorf("%s: %s: %w", op, err, errorsx.ErrInternal)
	}

	if pb.CandleTime == nil {
		return live.EnrichedSignal{}, fmt.Errorf("%s: candle_time is nil: %w", op, errorsx.ErrInternal)
	}

	if pb.CreatedAt == nil {
		return live.EnrichedSignal{}, fmt.Errorf("%s: created_at is nil: %w", op, errorsx.ErrInternal)
	}

	return live.EnrichedSignal{
		Signal: detect.Signal{
			ID:                id,
			RunID:             runID,
			CandleTime:        pb.CandleTime.AsTime(),
			BuyValue:          pb.BuyValue,
			TakeProfitValue:   pb.TakeProfitValue,
			StopLossValue:     pb.StopLossValue,
			ExpectedReturnPPM: pb.ExpectedReturnPpm,
			CreatedAt:         pb.CreatedAt.AsTime(),
		},
		Market:          marketSpec,
		Interval:        pb.Interval,
		DetectorCode:    pb.DetectorCode,
		DetectorVersion: pb.DetectorVersion,
	}, nil
}

func eventFromProto(pb *livepb.Event) (live.Event, error) {
	const op = "event from proto"
	if pb == nil {
		return live.Event{}, fmt.Errorf("%s: message is nil: %w", op, errorsx.ErrInternal)
	}

	switch v := pb.Payload.(type) {
	case *livepb.Event_RunStatus:
		event, err := runStatusEventFromProto(v)
		if err != nil {
			return live.Event{}, fmt.Errorf("%s: %w", op, err)
		}
		return live.Event{
			Payload: event,
		}, nil
	case *livepb.Event_Signal:
		s, err := signalFromProto(v.Signal)
		if err != nil {
			return live.Event{}, fmt.Errorf("%s: %w", op, err)
		}
		return live.Event{
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
		CreatedFrom:   xgrpc.TimePtrToProto(f.CreatedFrom),
		CreatedTo:     xgrpc.TimePtrToProto(f.CreatedTo),
	}
}

func runsPagedRequestToProto(req live.RunsPagedRequest) *livepb.ListRunsPagedRequest {
	return &livepb.ListRunsPagedRequest{
		Limit:       req.Limit,
		BeforeId:    xgrpc.UUIDPtrToProto(req.BeforeID),
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

	return &livepb.ListSignalsFilter{
		RunId:          xgrpc.UUIDPtrToProto(f.RunID),
		Categories:     f.Categories,
		Symbols:        f.Symbols,
		Intervals:      f.Intervals,
		DetectorsCodes: f.DetectorCodes,
		CreatedFrom:    xgrpc.TimePtrToProto(f.CreatedFrom),
		CreatedTo:      xgrpc.TimePtrToProto(f.CreatedTo),
	}
}

func signalsPagedRequestToProto(req live.SignalsPagedRequest) *livepb.ListSignalsPagedRequest {
	return &livepb.ListSignalsPagedRequest{
		Limit:    req.Limit,
		BeforeId: xgrpc.UUIDPtrToProto(req.BeforeID),
		Filter:   signalsFilterToProto(req.Filter),
	}
}

func signalsPagedResponseFromProto(pb *livepb.ListSignalsPagedResponse) (live.SignalsPagedResponse, error) {
	const op = "signals paged response from proto"
	if pb == nil {
		return live.SignalsPagedResponse{}, fmt.Errorf("%s: message is nil: %w", op, errorsx.ErrInternal)
	}

	signals := make([]live.EnrichedSignal, 0, len(pb.Signals))
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

func runStatusEventFromProto(pb *livepb.Event_RunStatus) (live.RunStatusEvent, error) {
	const op = "run status event from proto"
	if pb == nil {
		return live.RunStatusEvent{}, fmt.Errorf("%s: message is nil: %w", op, errorsx.ErrInternal)
	}

	id, err := uuid.Parse(pb.RunStatus.RunId)
	if err != nil {
		return live.RunStatusEvent{}, fmt.Errorf("%s: %w", errors.Join(errorsx.ErrInternal, err))
	}

	status, err := core.RunStatusFromProto(pb.RunStatus.Status)
	if err != nil {
		return live.RunStatusEvent{}, fmt.Errorf("%s: %w", op, err)
	}

	return live.RunStatusEvent{
		RunID:  id,
		Status: status,
	}, nil
}
