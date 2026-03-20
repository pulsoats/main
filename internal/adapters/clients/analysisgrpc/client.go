package analysisgrpc

import (
	"context"
	"fmt"
	"io"
	"strings"

	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/lib/errorsx"
	"github.com/pulsoats/main/internal/adapters/clients"
	"github.com/pulsoats/main/internal/domain/analysis"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type client struct {
	conn analysispb.AnalysisClient
}

func NewClient(addr string) (analysis.Client, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("analysis client: %w: grpc address", derrors.ErrRequired)
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("analysis client: %w: %v", errorsx.ErrInternal, err)
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

func (c *client) GetRunStatus(ctx context.Context, runID string) (analysis.RunStatus, error) {
	req := &analysispb.GetRunRequest{RunId: runID}

	resp, err := c.conn.GetRunStatus(ctx, req)
	if err != nil {
		return analysis.RunStatus{}, fmt.Errorf("get run status: %w", mapGRPCError(err))
	}

	status, ok := clients.MapRunStatusResponse(resp)
	if !ok {
		return analysis.RunStatus{}, fmt.Errorf("get run status: %w: resp is nil", derrors.ErrInvalidArgument)
	}
	return status, nil
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

func mapGRPCError(err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("%w: %v", errorsx.ErrInternal, err)
	}

	switch st.Code() {
	case codes.NotFound:
		return fmt.Errorf("%w: %s", derrors.ErrNotFound, st.Message())
	case codes.AlreadyExists, codes.Aborted:
		return fmt.Errorf("%w: %s", derrors.ErrAlreadyExists, st.Message())
	case codes.InvalidArgument, codes.OutOfRange, codes.FailedPrecondition:
		return fmt.Errorf("%w: %s", derrors.ErrInvalidArgument, st.Message())
	case codes.Unauthenticated, codes.PermissionDenied:
		return fmt.Errorf("%w: %s", derrors.ErrUnauthorized, st.Message())
	default:
		return fmt.Errorf("%w: %v", errorsx.ErrInternal, err)
	}
}
