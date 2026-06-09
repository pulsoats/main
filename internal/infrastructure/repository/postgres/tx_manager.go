package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pulsoats/core/errorsx"

	"github.com/pulsoats/main/internal/domain"
)

type txManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) domain.TxManager {
	return &txManager{pool: pool}
}

type txKey struct{}

func (m *txManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	const op = "within tx"
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	defer tx.Rollback(ctx)

	ctx = context.WithValue(ctx, txKey{}, tx)

	if err := fn(ctx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}
