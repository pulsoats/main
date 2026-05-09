package catalog

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"

	catalogpb "github.com/pulsoats/contracts/gen/go/catalog/v1"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	coregrpc "github.com/pulsoats/main/internal/adapters/grpc/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Client struct {
	conn catalogpb.CatalogClient
}

func NewClient(addr string, tlsCfg *tls.Config) (*Client, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("catalog client: addr: %w", errorsx.ErrInvalidArgument)
	}

	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("catalog client: addr: %w", errorsx.ErrInvalidArgument)
	}

	cred := credentials.TransportCredentials(insecure.NewCredentials())
	if tlsCfg != nil {
		cred = credentials.NewTLS(tlsCfg)
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(cred))
	if err != nil {
		return nil, fmt.Errorf("catalog client: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return &Client{conn: catalogpb.NewCatalogClient(conn)}, nil
}

func (c *Client) ListAvailableDetectors(ctx context.Context) ([]detect.DetectorMeta, error) {
	resp, err := c.conn.ListAvailableDetectors(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, coregrpc.MapError(err)
	}

	if resp == nil {
		return nil, fmt.Errorf("list available detectors: resp is nil: %w", errorsx.ErrInternal)
	}

	res := make([]detect.DetectorMeta, 0, len(resp.Detectors))
	for _, metaPb := range resp.Detectors {
		detMeta, ok := coregrpc.DetectorMetaFromProto(metaPb)
		if !ok {
			return nil, fmt.Errorf("list available detectors: meta is nil: %w", errorsx.ErrInternal)
		}
		res = append(res, detMeta)
	}

	return res, nil
}

func (c *Client) ListAvailableExchanges(ctx context.Context) ([]exchange.Meta, error) {
	resp, err := c.conn.ListAvailableExchanges(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, coregrpc.MapError(err)
	}

	if resp == nil {
		return nil, fmt.Errorf("list available exchanges: resp is nil: %w", errorsx.ErrInternal)
	}

	res := make([]exchange.Meta, 0, len(resp.ExchangeMetas))
	for _, metaPb := range resp.ExchangeMetas {
		exMeta, err := coregrpc.ExchangeMetaFromProto(metaPb)
		if err != nil {
			return nil, fmt.Errorf("list available exchanges: %w", errorsx.ErrInternal)
		}
		res = append(res, exMeta)
	}

	return res, nil
}
