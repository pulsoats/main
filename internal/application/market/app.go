package market

import (
	"context"
	"fmt"

	"github.com/pulsoats/core/domain/exchange"
	coremarket "github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchanges"
	"github.com/pulsoats/main/internal/domain"
	"github.com/pulsoats/main/internal/domain/market"
)

type Application struct {
	marketRepo market.Repository
	txManager  domain.TxManager
	exReg      *exchanges.Registry
	exToAPI    map[string]exchange.API
}

type ApplicationConfig struct {
	Repository        market.Repository
	TxManager         domain.TxManager
	ExchangesRegistry *exchanges.Registry
	exToMeta          map[string]exchange.Meta
}

func NewApplication(cfg ApplicationConfig) (*Application, error) {
	if cfg.Repository == nil {
		return nil, fmt.Errorf("market app: market repository: %w", errorsx.ErrInvalidArgument)
	}
	if cfg.TxManager == nil {
		return nil, fmt.Errorf("market app: tx (transaction) manager: %w", errorsx.ErrInvalidArgument)
	}
	if cfg.ExchangesRegistry == nil {
		return nil, fmt.Errorf("market app: exToMeta registry: %w", errorsx.ErrRequired)
	}

	exToAPI, err := cfg.ExchangesRegistry.CreateAllPublic()
	if err != nil {
		return nil, fmt.Errorf("market app: exchanges registry: %w", err)
	}

	return &Application{
		marketRepo: cfg.Repository,
		txManager:  cfg.TxManager,
		exReg:      cfg.ExchangesRegistry,
		exToAPI:    exToAPI,
	}, nil
}

func (s *Application) EnsureMarket(ctx context.Context, spec coremarket.Spec) error {
	err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		exists, err := s.marketRepo.Exists(txCtx, spec)
		if err != nil {
			return fmt.Errorf("ensure market: check exists: %w", err)
		}

		if exists {
			return nil
		}

		api, ok := s.exToAPI[spec.Exchange]
		if !ok {
			return fmt.Errorf("ensure market: exchange not found in exToAPI: %w", errorsx.ErrInternal)
		}

		supported, err := api.InstrumentExists(txCtx, spec.Category, spec.Symbol)
		if err != nil {
			return fmt.Errorf("ensure market: check instrument: %w", err)
		}
		if !supported {
			return fmt.Errorf("%s: instrument %s/%s unsupported",
				spec.Exchange,
				spec.Category,
				spec.Symbol,
			)
		}

		if err := s.marketRepo.CreateMarketSpec(txCtx, spec); err != nil {
			return fmt.Errorf("ensure market: create spec: %w", err)
		}
		return nil
	})

	return err
}

func (s *Application) ListExchangeMetas() []exchange.Meta {
	return s.exReg.ListMetadata()
}

func (s *Application) ListSymbols(ctx context.Context, exchange string, category coremarket.Category) ([]string, error) {
	return s.marketRepo.ListSymbols(ctx, exchange, category)
}
