package live

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/google/uuid"
	catalogpb "github.com/pulsoats/contracts/gen/go/catalog/v1"
	corepb "github.com/pulsoats/contracts/gen/go/core/v1"
	livepb "github.com/pulsoats/contracts/gen/go/live/v1"
	systempb "github.com/pulsoats/contracts/gen/go/system/v1"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	"github.com/pulsoats/core/market"
	coresystem "github.com/pulsoats/core/system"
	coregrpc "github.com/pulsoats/main/internal/adapters/grpc/core"
	grpcsystem "github.com/pulsoats/main/internal/adapters/grpc/system"
	"github.com/pulsoats/main/internal/domain/live"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Client struct {
	live    livepb.LiveClient
	catalog catalogpb.CatalogClient
	monitor systempb.ServiceMonitorClient
}

func NewClient(addr string, tlsCfg *tls.Config) (*Client, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("live client: addr: %w", errorsx.ErrInvalidArgument)
	}

	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("live client: addr: %w", errorsx.ErrInvalidArgument)
	}

	cred := credentials.TransportCredentials(insecure.NewCredentials())
	if tlsCfg != nil {
		cred = credentials.NewTLS(tlsCfg)
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(cred))
	if err != nil {
		return nil, fmt.Errorf("live client: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return &Client{
		live:    livepb.NewLiveClient(conn),
		catalog: catalogpb.NewCatalogClient(conn),
		monitor: systempb.NewServiceMonitorClient(conn),
	}, nil
}

func (c *Client) NewRun(ctx context.Context, market market.Spec, interval string, detector detect.DetectorConfig, callerID uuid.UUID) (live.Run, error) {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-user-id", callerID.String()))

	resp, err := c.live.NewRun(ctx, &livepb.NewRunRequest{
		Market:   coregrpc.MarketSpecToProto(market),
		Interval: interval,
		Detector: coregrpc.DetectorConfigToProto(detector),
	})
	if err != nil {
		return live.Run{}, coregrpc.MapError(err)
	}

	r, err := runFromProto(resp)
	if err != nil {
		return live.Run{}, fmt.Errorf("new run: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return r, nil
}

func (c *Client) StopRun(ctx context.Context, runID uuid.UUID, callerID uuid.UUID) error {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-user-id", callerID.String()))

	_, err := c.live.StopRun(ctx, &corepb.RunID{RunId: runID.String()})
	return coregrpc.MapError(err)
}

func (c *Client) StopAll(ctx context.Context, callerID uuid.UUID) error {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-user-id", callerID.String()))

	_, err := c.live.StopAll(ctx, &emptypb.Empty{})
	return coregrpc.MapError(err)
}

func (c *Client) RestartRun(ctx context.Context, runID uuid.UUID, callerID uuid.UUID) (live.Run, error) {
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-user-id", callerID.String()))

	resp, err := c.live.RestartRun(ctx, &corepb.RunID{RunId: runID.String()})
	if err != nil {
		return live.Run{}, coregrpc.MapError(err)
	}

	r, err := runFromProto(resp)
	if err != nil {
		return live.Run{}, fmt.Errorf("restart run: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return r, nil
}

func (c *Client) GetRun(ctx context.Context, runID uuid.UUID) (live.Run, error) {
	resp, err := c.live.GetRun(ctx, &corepb.RunID{RunId: runID.String()})
	if err != nil {
		return live.Run{}, coregrpc.MapError(err)
	}

	r, err := runFromProto(resp)
	if err != nil {
		return live.Run{}, fmt.Errorf("get run: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return r, nil
}

func (c *Client) StreamEvents(ctx context.Context) (*live.Stream, error) {
	grpcStream, err := c.live.StreamEvents(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, coregrpc.MapError(err)
	}

	ch := make(chan live.Event, 64)
	s := &live.Stream{Events: ch}

	go func() {
		defer close(ch)
		for {
			pb, err := grpcStream.Recv()
			if err != nil {
				if !errors.Is(err, io.EOF) && ctx.Err() == nil {
					s.Err = coregrpc.MapError(err)
				}
				return
			}
			event, err := eventFromProto(pb)
			if err != nil {
				s.Err = errors.Join(errorsx.ErrInternal, err)
				return
			}
			select {
			case ch <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	return s, nil
}

func (c *Client) ListRunsPaged(ctx context.Context, req live.ListRunsRequest) (live.ListRunsResponse, error) {
	pbReq := listRunsRequestToProto(req)
	pbResp, err := c.live.ListRunsPaged(ctx, pbReq)
	if err != nil {
		return live.ListRunsResponse{}, coregrpc.MapError(err)
	}

	resp, err := listRunsResponseFromProto(pbResp)
	if err != nil {
		return live.ListRunsResponse{}, fmt.Errorf("list runs paged: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return resp, nil
}

func (c *Client) ListSignalsPaged(ctx context.Context, req live.ListSignalsPagedRequest) (live.ListSignalsPagedResponse, error) {
	pbReq := listSignalsRequestToProto(req)
	pbResp, err := c.live.ListSignalsPaged(ctx, pbReq)
	if err != nil {
		return live.ListSignalsPagedResponse{}, fmt.Errorf("list signals paged: %w", coregrpc.MapError(err))
	}

	resp, err := listSignalsResponseFromProto(pbResp)
	if err != nil {
		return live.ListSignalsPagedResponse{}, fmt.Errorf("list signals paged: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return resp, nil
}

func (c *Client) ListAvailableDetectors(ctx context.Context) ([]detect.DetectorMeta, error) {
	resp, err := c.catalog.ListAvailableDetectors(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("list available detectors: %w", coregrpc.MapError(err))
	}

	if resp == nil {
		return nil, fmt.Errorf("list available detectors: resp is nil: %w", errorsx.ErrInternal)
	}

	result := make([]detect.DetectorMeta, 0, len(resp.Detectors))
	for _, pb := range resp.Detectors {
		meta, ok := coregrpc.DetectorMetaFromProto(pb)
		if !ok {
			return nil, fmt.Errorf("list available detectors: nil entry: %w", errorsx.ErrInternal)
		}
		result = append(result, meta)
	}

	return result, nil
}

func (c *Client) ListAvailableExchanges(ctx context.Context) ([]exchange.Meta, error) {
	resp, err := c.catalog.ListAvailableExchanges(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("list available exchanges: %w", coregrpc.MapError(err))
	}

	if resp == nil {
		return nil, fmt.Errorf("list available exchanges: resp is nil: %w", errorsx.ErrInternal)
	}

	result := make([]exchange.Meta, 0, len(resp.ExchangeMetas))
	for _, pb := range resp.ExchangeMetas {
		meta, err := coregrpc.ExchangeMetaFromProto(pb)
		if err != nil {
			return nil, fmt.Errorf("list available exchanges: %w", errorsx.ErrInternal)
		}
		result = append(result, meta)
	}

	return result, nil
}

func (c *Client) Info(ctx context.Context) (coresystem.ServiceInfo, error) {
	resp, err := c.monitor.Info(ctx, &emptypb.Empty{})
	if err != nil {
		return coresystem.ServiceInfo{}, fmt.Errorf("info: %w", coregrpc.MapError(err))
	}

	info, err := grpcsystem.ServiceInfoFromProto(resp)
	if err != nil {
		return coresystem.ServiceInfo{}, fmt.Errorf("info: %w: %w", errorsx.ErrInternal, err)
	}

	return info, nil
}

func (c *Client) Metrics(ctx context.Context) (coresystem.ServiceMetrics, error) {
	resp, err := c.monitor.Metrics(ctx, &emptypb.Empty{})
	if err != nil {
		return coresystem.ServiceMetrics{}, fmt.Errorf("metrics: %w", coregrpc.MapError(err))
	}

	metrics, err := grpcsystem.ServiceMetricsFromProto(resp)
	if err != nil {
		return coresystem.ServiceMetrics{}, fmt.Errorf("metrics: %w: %w", errorsx.ErrInternal, err)
	}

	return metrics, nil
}
