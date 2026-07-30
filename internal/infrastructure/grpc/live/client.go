package live

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	"github.com/google/uuid"
	catalogpb "github.com/pulsoats/contracts/gen/go/catalog/v1"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	livepb "github.com/pulsoats/contracts/gen/go/live/v1"
	"github.com/pulsoats/core/detect/detector"
	"github.com/pulsoats/core/detect/filter"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	"github.com/pulsoats/core/xgrpc"
	"github.com/pulsoats/main/internal/domain/live"
	"github.com/pulsoats/main/internal/infrastructure/grpc/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Client struct {
	live    livepb.LiveClient
	catalog catalogpb.CatalogClient
	health  grpc_health_v1.HealthClient
}

func NewClient(addr string, tlsCfg *tls.Config) (*Client, error) {
	const op = "live client"
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("%s: addr: %w", op, errorsx.ErrInvalidArgument)
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
		live:    livepb.NewLiveClient(conn),
		catalog: catalogpb.NewCatalogClient(conn),
		health:  grpc_health_v1.NewHealthClient(conn),
	}, nil
}

func (c *Client) HealthCheck(ctx context.Context) error {
	resp, err := c.health.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("not serving: %s", resp.Status)
	}
	return nil
}

func (c *Client) NewRun(ctx context.Context, callerID uuid.UUID, req live.NewRunRequest) (live.Run, error) {
	const op = "new run"
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-user-id", callerID.String()))

	filtersConfigs := make([]*corepb.FilterConfig, 0, len(req.FiltersConfigs))
	for _, fc := range req.FiltersConfigs {
		filtersConfigs = append(filtersConfigs, xgrpc.FilterConfigToProto(fc))
	}

	resp, err := c.live.NewRun(ctx, &livepb.NewRunRequest{
		Market:         xgrpc.MarketSpecToProto(req.MarketSpec),
		Interval:       req.Interval,
		DetectorConfig: xgrpc.DetectorConfigToProto(req.DetectorConfig),
		FiltersConfigs: filtersConfigs,
	})
	if err != nil {
		return live.Run{}, core.MapError(err)
	}

	r, err := runFromProto(resp)
	if err != nil {
		return live.Run{}, fmt.Errorf("%s: %w", op, err)
	}

	return r, nil
}

func (c *Client) StopRun(ctx context.Context, runID uuid.UUID, callerID uuid.UUID) error {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-user-id", callerID.String()))

	_, err := c.live.StopRun(ctx, &corepb.RunID{RunId: runID.String()})
	return core.MapError(err)
}

func (c *Client) StopAllRuns(ctx context.Context, callerID uuid.UUID) error {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-user-id", callerID.String()))

	_, err := c.live.StopAll(ctx, &emptypb.Empty{})
	return core.MapError(err)
}

func (c *Client) RestartRun(ctx context.Context, runID uuid.UUID, callerID uuid.UUID) (live.Run, error) {
	const op = "restart run"
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-user-id", callerID.String()))

	resp, err := c.live.RestartRun(ctx, &corepb.RunID{RunId: runID.String()})
	if err != nil {
		return live.Run{}, core.MapError(err)
	}

	r, err := runFromProto(resp)
	if err != nil {
		return live.Run{}, fmt.Errorf("%s: %w", op, err)
	}

	return r, nil
}

func (c *Client) RunByID(ctx context.Context, runID uuid.UUID) (live.Run, error) {
	const op = "get run"
	resp, err := c.live.GetRun(ctx, &corepb.RunID{RunId: runID.String()})
	if err != nil {
		return live.Run{}, core.MapError(err)
	}

	r, err := runFromProto(resp)
	if err != nil {
		return live.Run{}, fmt.Errorf("%s: %w", op, err)
	}

	return r, nil
}

func (c *Client) StreamEvents(ctx context.Context) (<-chan live.Event, error) {
	grpcStream, err := c.live.StreamEvents(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, core.MapError(err)
	}

	ch := make(chan live.Event, 64)

	go func() {
		defer close(ch)
		for {
			pb, err := grpcStream.Recv()
			if err != nil {
				if !errors.Is(err, io.EOF) && ctx.Err() == nil {
					slog.Error("stream events: recv", "error", err)
				}
				return
			}
			event, err := eventFromProto(pb)
			if err != nil {
				slog.Error("stream events: parse event", "error", err)
				return
			}
			select {
			case ch <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

func (c *Client) RunsPaged(ctx context.Context, req live.RunsPagedRequest) (live.RunsPagedResponse, error) {
	const op = "runs paged"
	pbReq := runsPagedRequestToProto(req)
	pbResp, err := c.live.ListRunsPaged(ctx, pbReq)
	if err != nil {
		return live.RunsPagedResponse{}, core.MapError(err)
	}

	resp, err := runsPagedResponseFromProto(pbResp)
	if err != nil {
		return live.RunsPagedResponse{}, fmt.Errorf("%s: %w", op, err)
	}

	return resp, nil
}

func (c *Client) SignalsPaged(ctx context.Context, req live.SignalsPagedRequest) (live.SignalsPagedResponse, error) {
	const op = "signals paged"
	pbReq := signalsPagedRequestToProto(req)
	pbResp, err := c.live.ListSignalsPaged(ctx, pbReq)
	if err != nil {
		return live.SignalsPagedResponse{}, fmt.Errorf("%s: %w", op, core.MapError(err))
	}

	resp, err := signalsPagedResponseFromProto(pbResp)
	if err != nil {
		return live.SignalsPagedResponse{}, fmt.Errorf("%s: %w", op, err)
	}

	return resp, nil
}

func (c *Client) AvailableDetectors(ctx context.Context) ([]detector.Meta, error) {
	const op = "available detectors"
	resp, err := c.catalog.ListAvailableDetectors(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, core.MapError(err))
	}

	result, err := core.AvailableDetectorsFromProto(resp)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return result, nil
}

func (c *Client) AvailableFilters(ctx context.Context) ([]filter.Meta, error) {
	const op = "available filters"
	resp, err := c.catalog.ListAvailableFilters(ctx, new(emptypb.Empty))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, core.MapError(err))
	}

	result, err := core.AvailableFiltersFromProto(resp)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return result, nil
}

func (c *Client) AvailableExchanges(ctx context.Context) ([]exchange.Meta, error) {
	const op = "available exchanges"
	resp, err := c.catalog.ListAvailableExchanges(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, core.MapError(err))
	}

	result, err := core.AvailableExchangesFromProto(resp)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return result, nil
}

func (c *Client) WorkerStats(ctx context.Context) (stats live.WorkerStats, err error) {
	const op = "worker stats"

	resp, err := c.live.GetWorkerStats(ctx, new(emptypb.Empty))
	if err != nil {
		return live.WorkerStats{}, fmt.Errorf("%s: %w", op, core.MapError(err))
	}

	return live.WorkerStats{
		ActiveRuns:   resp.ActiveRuns,
		SignalsTotal: resp.SignalsTotal,
	}, nil
}
