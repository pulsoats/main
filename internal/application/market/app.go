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

func (a *Application) Symbols(ctx context.Context, exchange, category string) ([]string, error) {
	return a.repo.Symbols(ctx, exchange, category)
}

func (a *Application) Suggest(ctx context.Context, exchange, query string, limit int) ([]market.Suggestion, error) {
	return a.repo.Suggest(ctx, exchange, query, limit)
}
