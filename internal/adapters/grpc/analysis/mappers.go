package analysis

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	"github.com/pulsoats/main/internal/adapters/grpc/core"
	"github.com/pulsoats/main/internal/domain/analysis"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapNewRunRequest(req analysis.NewRunRequest) *analysispb.NewRunRequest {
	return &analysispb.NewRunRequest{
		Market:   core.MarketSpecToProto(req.Market),
		Interval: req.Interval,
		From:     timestamppb.New(req.From),
		To:       timestamppb.New(req.To),
		Detector: core.DetectorConfigToProto(req.Detector),
		Fees:     core.FeesToProto(req.Fees),
	}
}

func runFromProto(pb *analysispb.Run) (analysis.Run, error) {
	if pb == nil {
		return analysis.Run{}, fmt.Errorf("resp is nil")
	}

	base, err := core.BaseRunFromProto(pb.BaseRun)
	if err != nil {
		return analysis.Run{}, err
	}

	out := analysis.Run{
		BaseRun:          base,
		AvgProfitPercent: pb.AvgProfitPercent,
		IsShared:         pb.IsShared,
		SharedAt:         timePtrFromProto(pb.GetSharedAt()),
	}

	return out, nil
}

func listRunsResponseFromProto(pb *analysispb.ListRunsResponse) (analysis.ListRunsPagedResponse, error) {
	if pb == nil {
		return analysis.ListRunsPagedResponse{}, fmt.Errorf("resp is nil")
	}

	runs := make([]analysis.Run, 0, len(pb.Runs))
	for i, r := range pb.GetRuns() {
		mapped, err := runFromProto(r)
		if err != nil {
			return analysis.ListRunsPagedResponse{}, fmt.Errorf("analysis list runs response.runs[%d]: %w", i, err)
		}
		runs = append(runs, mapped)
	}

	var nextBeforeID *uuid.UUID
	if pb.NextBeforeId != nil {
		id, err := uuid.Parse(*pb.NextBeforeId)
		if err != nil {
			return analysis.ListRunsPagedResponse{}, fmt.Errorf("invalid next before id: %w", err)
		}
		nextBeforeID = &id
	}

	resp := analysis.ListRunsPagedResponse{
		Runs:         runs,
		HasMore:      pb.HasMore,
		NextBeforeID: nextBeforeID,
	}

	return resp, nil
}

func timePtrFromProto(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}
