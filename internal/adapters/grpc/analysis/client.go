package analysis

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/google/uuid"
	analysispb "github.com/pulsoats/contracts/gen/go/analysis/v1"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/adapters/grpc/core"
	"github.com/pulsoats/main/internal/domain/analysis"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type Client struct {
	conn analysispb.AnalysisClient
}

func NewClient(addr string, tlsCfg *tls.Config) (*Client, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("analysis client: empty addr")
	}

	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("analysis client: addr: %w", errorsx.ErrInvalidArgument)
	}

	cred := credentials.TransportCredentials(insecure.NewCredentials())
	if tlsCfg != nil {
		cred = credentials.NewTLS(tlsCfg)
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(cred))
	if err != nil {
		return nil, fmt.Errorf("analysis client: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return &Client{conn: analysispb.NewAnalysisClient(conn)}, nil
}

func (c *Client) NewRun(ctx context.Context, callerID uuid.UUID, req analysis.NewRunRequest) (analysis.Run, error) {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"x-user-id", callerID.String(),
	))

	run, err := c.conn.NewRun(ctx, mapNewRunRequest(req))
	if err != nil {
		return analysis.Run{}, core.MapError(err)
	}
	return runFromProto(run)
}

func (c *Client) RunByID(ctx context.Context, runID uuid.UUID) (analysis.Run, error) {
	run, err := c.conn.GetRun(ctx, &corepb.RunID{RunId: runID.String()})
	if err != nil {
		return analysis.Run{}, core.MapError(err)
	}
	return runFromProto(run)
}

func (c *Client) StreamRunArchive(ctx context.Context, runID uuid.UUID, dst io.Writer) error {
	stream, err := c.conn.GetRunArchive(ctx, &corepb.RunID{RunId: runID.String()})
	if err != nil {
		return core.MapError(err)
	}

	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return core.MapError(err)
		}
		if chunk == nil {
			return fmt.Errorf("run archive chunk is nil: %w", errorsx.ErrInternal)
		}
		if len(chunk.GetData()) == 0 {
			continue
		}
		if _, err := dst.Write(chunk.GetData()); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

func (c *Client) ShareRun(ctx context.Context, callerID, runID uuid.UUID) error {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"x-user-id", callerID.String(),
	))

	_, err := c.conn.ShareRun(ctx, &corepb.RunID{RunId: runID.String()})
	return core.MapError(err)
}

func (c *Client) DeleteRun(ctx context.Context, callerID, runID uuid.UUID) error {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"x-user-id", callerID.String(),
	))

	_, err := c.conn.DeleteRun(ctx, &corepb.RunID{RunId: runID.String()})
	return core.MapError(err)
}

func (c *Client) ListRunsPaged(ctx context.Context, callerID uuid.UUID, req analysis.ListRunsPagedRequest) (analysis.ListRunsPagedResponse, error) {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"x-user-id", callerID.String(),
	))

	var beforeIDPtr *string
	if req.BeforeID != nil {
		s := req.BeforeID.String()
		beforeIDPtr = &s
	}

	resp, err := c.conn.ListRunsPaged(ctx, &analysispb.ListRunsRequest{
		Limit:    req.Limit,
		BeforeId: beforeIDPtr,
		Filter:   analysispb.RunFilter(req.Filter),
	})
	if err != nil {
		return analysis.ListRunsPagedResponse{}, core.MapError(err)
	}

	lsResp, err := listRunsResponseFromProto(resp)
	if err != nil {
		return analysis.ListRunsPagedResponse{}, err
	}

	return lsResp, nil
}
