package market

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pulsoats/core/domain/exchange"
	coremarket "github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/domain/market"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres"
)

type repo struct {
	qp postgres.QuerierProvider
}

func NewPostgresRepository(qp postgres.QuerierProvider) market.Repository {
	return &repo{qp: qp}
}

func (r *repo) SyncExchanges(ctx context.Context, metas []exchange.Meta) error {
	if len(metas) == 0 {
		return nil
	}

	values := make([]string, 0, len(metas))
	args := make([]any, 0, len(metas))
	for i, meta := range metas {
		code := strings.TrimSpace(meta.Code)
		if code == "" {
			return fmt.Errorf("sync exchanges: %w: empty exchange code", errorsx.ErrInvalidArgument)
		}
		values = append(values, fmt.Sprintf("($%d)", i+1))
		args = append(args, code)
	}

	query := `
		INSERT INTO exchanges (code)
		VALUES ` + strings.Join(values, ",") + `
		ON CONFLICT (code) DO NOTHING;
	`

	q := r.qp.Get(ctx)
	if _, err := q.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("sync exchanges: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}

func (r *repo) Exists(ctx context.Context, spec coremarket.Spec) (bool, error) {
	var exists bool

	q := r.qp.Get(ctx)
	err := q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM markets m
			JOIN exchanges e ON e.id = m.exchange_id
			WHERE e.code = $1
			  AND m.category = $2
			  AND m.symbol = $3
			  AND m.is_active = true
		)
	`,
		spec.Exchange,
		spec.Category,
		spec.Symbol,
	).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("market exists: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return exists, nil
}

func (r *repo) CreateMarketSpec(ctx context.Context, spec coremarket.Spec) error {
	q := r.qp.Get(ctx)
	_, err := q.Exec(ctx, `
		INSERT INTO markets (exchange_id, category, symbol)
		SELECT e.id, $2, $3
		FROM exchanges e
		WHERE e.code = $1
	`,
		spec.Exchange,
		spec.Category,
		spec.Symbol,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				// UNIQUE (exchange_id, category, symbol)
				// рынок уже создан параллельно — считаем успехом
				return nil
			case "23503":
				return fmt.Errorf("create market spec: exchange %s: %w", spec.Exchange, errorsx.ErrNotFound)

			case "23502":
				return fmt.Errorf("create market spec: %w", errorsx.ErrInvalidArgument)

			case "22P02":
				return fmt.Errorf("create market spec: %w", errorsx.ErrInvalidArgument)

			case "40001", "40P01":
				// serialization failure / deadlock
				return fmt.Errorf("create market spec: retry transaction: %w", err)
			default:
				return fmt.Errorf("create market spec: %w", errors.Join(errorsx.ErrInternal, err))
			}
		}

		return fmt.Errorf("create market spec: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return nil
}

func (r *repo) ListSymbols(ctx context.Context, exchange string, category coremarket.Category) ([]string, error) {
	const query = `
		SELECT m.symbol
		FROM markets m
		JOIN exchanges e ON e.id = m.exchange_id
		WHERE e.code = $1
		  AND m.category = $2
		  AND m.is_active = TRUE
		ORDER BY m.symbol;
	`

	q := r.qp.Get(ctx)
	rows, err := q.Query(ctx, query, exchange, category)
	if err != nil {
		return nil, fmt.Errorf("list symbols: %w", errors.Join(errorsx.ErrInternal, err))
	}

	symbols, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (string, error) {
		var symbol string
		err := r.Scan(&symbol)
		if err != nil {
			return "", err
		}

		return symbol, nil
	})
	if err != nil {
		return nil, fmt.Errorf("list symbols: %w", errors.Join(errorsx.ErrInternal, err))
	}

	slices.Sort(symbols)
	return symbols, nil
}

func (r *repo) Suggest(ctx context.Context, exchange string, query string, limit int) ([]market.Suggestion, error) {
	if limit <= 0 {
		limit = 20
	}

	const suggestQuery = `
		SELECT m.category, m.symbol
		FROM markets m
		JOIN exchanges e ON e.id = m.exchange_id
		WHERE e.code = $1
		  AND m.is_active = TRUE
		  AND (
		  	  m.symbol ILIKE '%' || $2 || '%'
		  	  OR m.category ILIKE '%' || $2 || '%'
		  )
		ORDER BY m.symbol
		LIMIT $3;
	`

	q := r.qp.Get(ctx)
	rows, err := q.Query(ctx, suggestQuery, exchange, query, limit)
	if err != nil {
		return nil, fmt.Errorf("suggest markets: %w", errors.Join(errorsx.ErrInternal, err))
	}
	defer rows.Close()

	suggestions, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (market.Suggestion, error) {
		var (
			category string
			symbol   string
		)
		if err := r.Scan(&category, &symbol); err != nil {
			return market.Suggestion{}, err
		}
		return market.Suggestion{
			Category: coremarket.Category(category),
			Symbol:   symbol,
		}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("suggest markets: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return suggestions, nil
}
