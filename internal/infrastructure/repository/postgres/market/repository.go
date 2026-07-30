package market

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/domain/market"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres"
)

type Repository struct {
	qp postgres.QuerierProvider
}

func NewPostgresRepository(qp postgres.QuerierProvider) *Repository {
	return &Repository{qp: qp}
}

func (r *Repository) UpsertSymbols(ctx context.Context, exchange, category string, symbols []string) error {
	if len(symbols) == 0 {
		return nil
	}

	const query = `
	INSERT INTO markets (exchange, category, symbol)
	VALUES ($1, $2, $3)
	ON CONFLICT (exchange, category, symbol) DO NOTHING;
	`

	q := r.qp.Get(ctx)

	for _, symbol := range symbols {
		if _, err := q.Exec(ctx, query, exchange, category, symbol); err != nil {
			return fmt.Errorf("upsert symbols: %s/%s: %w", exchange, category, errors.Join(errorsx.ErrInternal, err))
		}
	}

	return nil
}

func (r *Repository) Symbols(ctx context.Context, exchange, category string) ([]string, error) {
	const query = `
	SELECT DISTINCT symbol
	FROM markets
	WHERE exchange = $1
	  AND category = $2
	ORDER BY symbol;
	`

	q := r.qp.Get(ctx)

	rows, err := q.Query(ctx, query, exchange, category)
	if err != nil {
		return nil, fmt.Errorf("list symbols: %w", errors.Join(errorsx.ErrInternal, err))
	}
	defer rows.Close()

	symbols, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (string, error) {
		var s string
		return s, row.Scan(&s)
	})
	if err != nil {
		return nil, fmt.Errorf("list symbols: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return symbols, nil
}

func (r *Repository) Suggest(ctx context.Context, exchange, query string, limit int) ([]market.Suggestion, error) {
	if limit <= 0 {
		limit = 20
	}

	const q = `
	SELECT DISTINCT category, symbol
	FROM markets
	WHERE exchange = $1
	  AND (
	      symbol   ILIKE '%' || $2 || '%'
	      OR category ILIKE '%' || $2 || '%'
	  )
	ORDER BY symbol
	LIMIT $3;
	`

	qr := r.qp.Get(ctx)

	rows, err := qr.Query(ctx, q, exchange, query, limit)
	if err != nil {
		return nil, fmt.Errorf("suggest: %w", errors.Join(errorsx.ErrInternal, err))
	}
	defer rows.Close()

	suggestions, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (market.Suggestion, error) {
		var s market.Suggestion
		s.Exchange = exchange
		return s, row.Scan(&s.Category, &s.Symbol)
	})
	if err != nil {
		return nil, fmt.Errorf("suggest: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return suggestions, nil
}
