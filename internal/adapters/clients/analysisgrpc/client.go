package analysisgrpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/domain/analysis"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type client struct {
	conn analysispb.AnalysisClient
}

func NewClient(addr string) (analysis.Client, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("analysis client: grpc address: %w", errorsx.ErrInvalidArgument)
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("analysis client: %w: %v", errors.Join(errorsx.ErrInternal, err))
	}

	return &client{
		conn: analysispb.NewAnalysisClient(conn),
	}, nil
}

func (c *client) StartRun(ctx context.Context, request analysis.StartRunRequest) (string, error) {
	req, err := mapRequestToGRPC(request)
	if err != nil {
		return "", fmt.Errorf("start run: %w", err)
	}

	resp, err := c.conn.StartRun(ctx, req)
	if err != nil {
		return "", fmt.Errorf("start run: %w", mapGRPCError(err))
	}

	return resp.RunId, nil
}

func (c *client) GetRunMeta(ctx context.Context, runID string) (analysis.Run, error) {
	req := analysispb.GetRunRequest{RunId: runID}
	rawMeta, err := c.conn.GetRunMeta(ctx, &req)
	if err != nil {
		return analysis.Run{}, fmt.Errorf("get run meta: %w", mapGRPCError(err))
	}

	meta, err := mapRunMetaResponse(rawMeta)
	if err != nil {
		return analysis.Run{}, fmt.Errorf("get run meta: %w", err)
	}

	return meta, nil
}

func (c *client) GetRunResult(ctx context.Context, runID string) (chan []byte, error) {
	req := &analysispb.GetRunRequest{RunId: runID}

	stream, err := c.conn.GetRunResult(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get run result: %w", mapGRPCError(err))
	}

	out := make(chan []byte, 16)

	go func() {
		defer close(out)

		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				return
			}

			if len(chunk.GetData()) == 0 {
				continue
			}

			select {
			case out <- chunk.Data:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

func (c *client) ShareRun(ctx context.Context, userID uuid.UUID, runID string) error {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("user-id", userID.String()))
	_, err := c.conn.ShareRun(ctx, &analysispb.ShareRunRequest{RunId: runID})
	if err != nil {
		return fmt.Errorf("share run: %w", mapGRPCError(err))
	}
	return nil
}

func (c *client) ListRunsPaged(ctx context.Context, userID uuid.UUID, limit int, beforeID *int64, filter string) (analysis.RunsPage, error) {
	if limit <= 0 {
		limit = 20
	}

	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("user-id", userID.String()))

	req := &analysispb.ListRunsRequest{
		Limit:  int32(limit),
		Filter: mapRunFilter(filter),
	}
	if beforeID != nil {
		req.BeforeId = *beforeID
	}

	resp, err := c.conn.ListRunsPaged(ctx, req)
	if err != nil {
		return analysis.RunsPage{}, fmt.Errorf("list runs paged: %w", mapGRPCError(err))
	}

	runs := make([]analysis.Run, 0, len(resp.GetItems()))
	for _, raw := range resp.GetItems() {
		meta, err := mapRunMetaResponse(raw)
		if err != nil {
			return analysis.RunsPage{}, fmt.Errorf("list runs paged: %w", err)
		}
		runs = append(runs, meta)
	}

	var nextBeforeID *int64
	if resp.GetNextBeforeId() != 0 {
		next := resp.GetNextBeforeId()
		nextBeforeID = &next
	}

	return analysis.RunsPage{
		Items:        runs,
		NextBeforeID: nextBeforeID,
		HasMore:      resp.GetHasMore(),
	}, nil
}

func mapGRPCError(err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("%w: %v", errors.Join(errorsx.ErrInternal, err))
	}

	switch st.Code() {
	case codes.NotFound:
		return fmt.Errorf("%w: %s", errorsx.ErrNotFound, st.Message())
	case codes.AlreadyExists, codes.Aborted:
		return fmt.Errorf("%w: %s", errorsx.ErrAlreadyExists, st.Message())
	case codes.InvalidArgument, codes.OutOfRange, codes.FailedPrecondition:
		return fmt.Errorf("%w: %s", errorsx.ErrInvalidArgument, st.Message())
	case codes.Unauthenticated, codes.PermissionDenied:
		return fmt.Errorf("%w: %s", errorsx.ErrUnauthorized, st.Message())
	default:
		return fmt.Errorf("%w: %v", errors.Join(errorsx.ErrInternal, err))
	}
}
