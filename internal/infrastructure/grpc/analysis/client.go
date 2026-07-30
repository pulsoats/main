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
	catalogpb "github.com/pulsoats/contracts/gen/go/catalog/v1"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	"github.com/pulsoats/core/detect/detector"
	"github.com/pulsoats/core/detect/filter"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	"github.com/pulsoats/main/internal/domain/analysis"
	coregrpc "github.com/pulsoats/main/internal/infrastructure/grpc/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Client struct {
	analysis analysispb.AnalysisClient
	catalog  catalogpb.CatalogClient
}

func NewClient(addr string, tlsCfg *tls.Config) (*Client, error) {
	const op = "analysis client"
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("%s: empty addr", op)
	}

	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("%s: addr: %w", op, errorsx.ErrInvalidArgument)
	}

	cred := insecure.NewCredentials()
	if tlsCfg != nil {
		cred = credentials.NewTLS(tlsCfg)
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(cred))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return &Client{
		analysis: analysispb.NewAnalysisClient(conn),
		catalog:  catalogpb.NewCatalogClient(conn),
	}, nil
}

func (c *Client) NewRun(ctx context.Context, userID uuid.UUID, req analysis.NewRunRequest) (analysis.Run, error) {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"x-user-id", userID.String(),
	))

	run, err := c.analysis.NewRun(ctx, mapNewRunRequest(req))
	if err != nil {
		return analysis.Run{}, coregrpc.MapError(err)
	}
	return runFromProto(run)
}

func (c *Client) RunByID(ctx context.Context, runID uuid.UUID) (analysis.Run, error) {
	const op = "run by id"
	resp, err := c.analysis.GetRun(ctx, &corepb.RunID{RunId: runID.String()})
	if err != nil {
		return analysis.Run{}, coregrpc.MapError(fmt.Errorf("%s: %w", op, err))
	}

	run, err := runFromProto(resp)
	if err != nil {
		return analysis.Run{}, fmt.Errorf("%s: %w", op, err)
	}

	return run, nil
}

func (c *Client) StreamRunArchive(ctx context.Context, runID uuid.UUID, dst io.Writer) error {
	stream, err := c.analysis.GetRunArchive(ctx, &corepb.RunID{RunId: runID.String()})
	if err != nil {
		return coregrpc.MapError(err)
	}

	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return coregrpc.MapError(err)
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

func (c *Client) ShareRun(ctx context.Context, userID, runID uuid.UUID) error {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"x-user-id", userID.String(),
	))

	_, err := c.analysis.ShareRun(ctx, &corepb.RunID{RunId: runID.String()})
	return coregrpc.MapError(err)
}

func (c *Client) DeleteRun(ctx context.Context, userID, runID uuid.UUID) error {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"x-user-id", userID.String(),
	))

	_, err := c.analysis.DeleteRun(ctx, &corepb.RunID{RunId: runID.String()})
	return coregrpc.MapError(err)
}

func (c *Client) RunsPaged(ctx context.Context, userID uuid.UUID, req analysis.RunsPagedRequest) (analysis.RunsPagedResponse, error) {
	const op = "runs paged"
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"x-user-id", userID.String(),
	))

	var beforeID *string
	if req.BeforeID != nil {
		beforeID = new(req.BeforeID.String())
	}

	var scope analysispb.RunScope
	switch req.Scope {
	case 1:
		scope = analysispb.RunScope_RUN_SCOPE_MINE
	case 2:
		scope = analysispb.RunScope_RUN_SCOPE_SHARED
	case 3:
		scope = analysispb.RunScope_RUN_SCOPE_ALL
	default:
		scope = analysispb.RunScope_RUN_SCOPE_UNSPECIFIED
	}

	resp, err := c.analysis.ListRunsPaged(ctx, &analysispb.ListRunsPagedRequest{
		Limit:       req.Limit,
		BeforeId:    beforeID,
		OrderDirAsc: req.OrderDirAsc,
		Scope:       scope,
		Filter:      runsFilterToProto(req.Filter),
	})
	if err != nil {
		return analysis.RunsPagedResponse{}, coregrpc.MapError(err)
	}

	respFromProto, err := runsPagedResponseFromProto(resp)
	if err != nil {
		return analysis.RunsPagedResponse{}, fmt.Errorf("%s: %w", op, err)
	}

	return respFromProto, nil
}

func (c *Client) AvailableDetectors(ctx context.Context) ([]detector.Meta, error) {
	const op = "list available detectors"
	resp, err := c.catalog.ListAvailableDetectors(ctx, new(emptypb.Empty))
	if err != nil {
		return nil, coregrpc.MapError(err)
	}
	if resp == nil {
		return nil, fmt.Errorf("%s: resp is nil: %w", op, errorsx.ErrInternal)
	}

	res, err := coregrpc.AvailableDetectorsFromProto(resp)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return res, nil
}

func (c *Client) AvailableFilters(ctx context.Context) ([]filter.Meta, error) {
	const op = "available filters"
	resp, err := c.catalog.ListAvailableFilters(ctx, new(emptypb.Empty))
	if err != nil {
		return nil, coregrpc.MapError(err)
	}
	if resp == nil {
		return nil, fmt.Errorf("%s: nil response: %w", op, errorsx.ErrInternal)
	}

	return coregrpc.AvailableFiltersFromProto(resp)
}

func (c *Client) AvailableExchanges(ctx context.Context) ([]exchange.Meta, error) {
	const op = "list available exchanges"
	resp, err := c.catalog.ListAvailableExchanges(ctx, new(emptypb.Empty))
	if err != nil {
		return nil, coregrpc.MapError(err)
	}
	if resp == nil {
		return nil, fmt.Errorf("%s: resp is nil: %w", op, errorsx.ErrInternal)
	}

	res, err := coregrpc.AvailableExchangesFromProto(resp)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return res, nil
}
