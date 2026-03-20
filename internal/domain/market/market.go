package market

import (
	"context"

	"github.com/pulsoats/core/domain/exchange"
	"github.com/pulsoats/core/domain/market"
)

type Repository interface {
	SyncExchanges(ctx context.Context, metas []exchange.Meta) error
	CreateMarketSpec(ctx context.Context, spec market.Spec) error
	Exists(ctx context.Context, spec market.Spec) (bool, error)
	ListSymbols(ctx context.Context, exchange string, category market.Category) ([]string, error)
	Suggest(ctx context.Context, exchange string, query string, limit int) ([]Suggestion, error)
}

type Service interface {
	EnsureMarket(ctx context.Context, spec market.Spec) error
	ListExchangeMetas() []exchange.Meta
	ListSymbols(ctx context.Context, exchange string, category market.Category) ([]string, error)
}

type Suggestion struct {
	Category market.Category
	Symbol   string
}
