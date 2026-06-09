package market

import "context"

type Suggestion struct {
	Exchange string
	Category string
	Symbol   string
}

type Repository interface {
	UpsertSymbols(ctx context.Context, exchange, category string, symbols []string) error
	Symbols(ctx context.Context, exchange, category string) ([]string, error)
	Suggest(ctx context.Context, exchange, query string, limit int) ([]Suggestion, error)
}
