package market

import (
	"context"

	"github.com/pulsoats/main/internal/domain/market"
)

type Application struct {
	repo market.Repository
}

func NewApplication(repo market.Repository) *Application {
	return &Application{repo: repo}
}

func (a *Application) ListSymbols(ctx context.Context, exch, category string) ([]string, error) {
	return a.repo.ListSymbols(ctx, exch, category)
}

func (a *Application) Suggest(ctx context.Context, exch, query string, limit int) ([]market.Suggestion, error) {
	return a.repo.Suggest(ctx, exch, query, limit)
}
