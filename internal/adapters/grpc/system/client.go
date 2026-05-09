package health

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"

	systempb "github.com/pulsoats/contracts/gen/go/system/v1"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/system"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Client struct {
	conn systempb.ServiceMonitorClient
}

func NewClient(addr string, tlsCfg *tls.Config) (*Client, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("health client: empty addr")
	}

	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("health client: addr: %w", errorsx.ErrInvalidArgument)
	}

	cred := credentials.TransportCredentials(insecure.NewCredentials())
	if tlsCfg != nil {
		cred = credentials.NewTLS(tlsCfg)
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(cred))
	if err != nil {
		return nil, fmt.Errorf("health client: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return &Client{conn: systempb.NewServiceMonitorClient(conn)}, nil
}

func (c *Client) Info(ctx context.Context) (system.ServiceInfo, error) {
	resp, err := c.conn.Info(ctx, &emptypb.Empty{})
	if err != nil {
		return system.ServiceInfo{}, fmt.Errorf("info: %w", errors.Join(errorsx.ErrInternal, err))
	}

	info, err := ServiceInfoFromProto(resp)
	if err != nil {
		return system.ServiceInfo{}, fmt.Errorf("info: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return info, nil
}

func (c *Client) Metrics(ctx context.Context) (system.ServiceMetrics, error) {
	resp, err := c.conn.Metrics(ctx, &emptypb.Empty{})
	if err != nil {
		return system.ServiceMetrics{}, fmt.Errorf("metrics: %w", errors.Join(errorsx.ErrInternal, err))
	}

	metrics, err := ServiceMetricsFromProto(resp)
	if err != nil {
		return system.ServiceMetrics{}, fmt.Errorf("metrics: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return metrics, nil
}
