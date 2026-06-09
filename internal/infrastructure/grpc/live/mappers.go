package live

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	livepb "github.com/pulsoats/contracts/gen/go/live/v1"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/domain/live"
	"github.com/pulsoats/main/internal/infrastructure/grpc/core"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func runFromProto(pb *livepb.Run) (live.Run, error) {
	if pb == nil {
		return live.Run{}, errors.New("nil pb")
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

	return live.Run{
		Base:             baseRun,
		OrdersCount:      pb.GetOrdersCount(),
		SumProfitPercent: pb.SumProfitPercent,
		FinishedAt:       finishedAt,
		FinishedBy:       pb.FinishedBy,
	}, nil
}

func signalFromProto(pb *livepb.Signal) (detect.Signal, error) {
	if pb == nil {
		return detect.Signal{}, errors.New("nil pb")
	}
	id, err := uuid.Parse(pb.Id)
	if err != nil {
		return detect.Signal{}, fmt.Errorf("id: %w", err)
	}

	runID, err := uuid.Parse(pb.RunId)
	if err != nil {
		return detect.Signal{}, fmt.Errorf("run_id: %w", err)
	}

	market, ok := core.MarketSpecFromProto(pb.Market)
	if !ok {
		return detect.Signal{}, fmt.Errorf("market: %w", errorsx.ErrInvalidArgument)
	}

	if pb.Time == nil {
		return detect.Signal{}, errors.New("time is nil")
	}

	fingerprint, err := uuid.Parse(pb.Fingerprint)
	if err != nil {
		return detect.Signal{}, fmt.Errorf("fingerprint: %w", err)
	}

	if pb.CreatedAt == nil {
		return detect.Signal{}, errors.New("created_at is nil")
	}

	return detect.Signal{
		ID:                id,
		RunID:             runID,
		Market:            market,
		DetectorCode:      pb.GetDetectorCode(),
		DetectorOptsLabel: pb.GetDetectorOptsLabel(),
		Time:              pb.Time.AsTime().UnixMilli(),
		Value:             pb.Value,
		BuyValue:          pb.BuyValue,
		TakeProfitValue:   pb.TakeProfitValue,
		StopLossValue:     pb.StopLossValue,
		ExpectedReturnPPM: pb.ExpectedReturnPpm,
		Fingerprint:       fingerprint,
		CreatedAt:         pb.CreatedAt.AsTime().UnixMilli(),
	}, nil
}

func eventFromProto(pb *livepb.Event) (live.Event, error) {
	if pb == nil {
		return live.Event{}, errors.New("resp is nil")
	}

	runID, err := uuid.Parse(pb.RunId)
	if err != nil {
		return live.Event{}, fmt.Errorf("run_id: %w", err)
	}
	switch p := pb.Payload.(type) {
	case *livepb.Event_Run:
		r, err := runFromProto(p.Run)
		if err != nil {
			return live.Event{}, err
		}
		return live.Event{
			RunID:   runID,
			Payload: live.RunEvent{Run: r},
		}, nil
	case *livepb.Event_Signal:
		s, err := signalFromProto(p.Signal)
		if err != nil {
			return live.Event{}, err
		}
		return live.Event{
			RunID:   runID,
			Payload: live.SignalEvent{Signal: s},
		}, nil
	default:
		return live.Event{}, errors.New("unknown event type")
	}
}

func listRunsFilterToProto(filter *live.RunsFilter) *livepb.ListRunsFilter {
	if filter == nil {
		return nil
	}

	var pb livepb.ListRunsFilter

	if filter.StatusCode != nil {
		pbCode := corepb.RunStatusCode(*filter.StatusCode)
		pb.StatusCode = &pbCode
	}

	if filter.Interval != nil {
		s := filter.Interval.String()
		pb.Interval = &s
	}

	pb.Category = filter.Category
	pb.Symbol = filter.Symbol
	pb.DetectorCode = filter.DetectorCode
	pb.OrderDirAsc = filter.OrderDirAsc
	return &pb
}

func listRunsRequestToProto(req live.RunsRequest) *livepb.ListRunsPagedRequest {
	return &livepb.ListRunsPagedRequest{
		Limit: req.Limit,
		BeforeId: func() *string {
			if req.BeforeID != nil {
				s := req.BeforeID.String()
				return &s
			}
			return nil
		}(),
		Filter: listRunsFilterToProto(req.Filter),
	}
}

func listRunsResponseFromProto(pb *livepb.ListRunsPagedResponse) (live.RunsResponse, error) {
	if pb == nil {
		return live.RunsResponse{}, errors.New("resp is nil")
	}

	runs := make([]live.Run, 0, len(pb.Runs))
	for _, runPb := range pb.Runs {
		sig, err := runFromProto(runPb)
		if err != nil {
			return live.RunsResponse{}, err
		}
		runs = append(runs, sig)
	}

	var nextBeforeID *uuid.UUID
	if pb.NextBeforeId != nil {
		id, err := uuid.Parse(pb.GetNextBeforeId())
		if err != nil {
			return live.RunsResponse{}, fmt.Errorf("nextBeforeId: %w", err)
		}
		nextBeforeID = &id
	}

	return live.RunsResponse{
		Runs:         runs,
		HasMore:      pb.HasMore,
		NextBeforeID: nextBeforeID,
	}, nil
}

func listSignalsFilterToProto(filter *live.SignalsFilter) *livepb.ListSignalsFilter {
	if filter == nil {
		return nil
	}

	var pb livepb.ListSignalsFilter

	if filter.RunID != nil {
		s := filter.RunID.String()
		pb.RunId = &s
	}

	pb.Category = filter.Category
	pb.Symbol = filter.Symbol

	if filter.From != nil {
		pb.From = timestamppb.New(*filter.From)
	}
	if filter.To != nil {
		pb.To = timestamppb.New(*filter.To)
	}

	pb.OrderDirAsc = filter.OrderDirAsc
	return &pb
}

func listSignalsRequestToProto(req live.SignalsPagedRequest) *livepb.ListSignalsPagedRequest {
	return &livepb.ListSignalsPagedRequest{
		Limit: req.Limit,
		BeforeId: func() *string {
			if req.BeforeID != nil {
				s := req.BeforeID.String()
				return &s
			}
			return nil
		}(),
		Filter: listSignalsFilterToProto(req.Filter),
	}
}

func listSignalsResponseFromProto(pb *livepb.ListSignalsPagedResponse) (live.SignalsPagedResponse, error) {
	if pb == nil {
		return live.SignalsPagedResponse{}, errors.New("pb is nil")
	}

	signals := make([]detect.Signal, 0, len(pb.Signals))
	for _, sigPb := range pb.Signals {
		sig, err := signalFromProto(sigPb)
		if err != nil {
			return live.SignalsPagedResponse{}, err
		}
		signals = append(signals, sig)
	}

	var nextBeforeID *uuid.UUID
	if pb.NextBeforeId != nil {
		id, err := uuid.Parse(pb.GetNextBeforeId())
		if err != nil {
			return live.SignalsPagedResponse{}, fmt.Errorf("nextBeforeId: %w", err)
		}
		nextBeforeID = &id
	}

	return live.SignalsPagedResponse{
		Signals:      signals,
		HasMore:      pb.HasMore,
		NextBeforeID: nextBeforeID,
	}, nil
}
