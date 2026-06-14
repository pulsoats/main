package live

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/domain/live"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres"
)

type WorkerRepository struct {
	qp postgres.QuerierProvider
}

func NewPostgresWorkerRepository(qp postgres.QuerierProvider) *WorkerRepository {
	return &WorkerRepository{qp: qp}
}

func (r *WorkerRepository) CreateWorker(ctx context.Context, worker *live.Worker) error {
	const op = "create worker"
	const query = `
	INSERT INTO workers (id, node_id, host, grpc_port, container_id, exchange_account_id, status, last_error)
	VALUES($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING created_at;`

	q := r.qp.Get(ctx)

	err := q.QueryRow(ctx, query,
		worker.ID,
		worker.NodeID,
		worker.Host,
		worker.GRPCPort,
		worker.ContainerID,
		worker.ExchangeAccountID,
		worker.Status,
		worker.LastError,
	).Scan(&worker.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			switch pgErr.ConstraintName {
			case "workers_node_id_fkey":
				return fmt.Errorf("%s: node: %w", op, errorsx.ErrNotFound)
			case "workers_exchange_account_id_fkey":
				return fmt.Errorf("%s: exchange account: %w", op, errorsx.ErrNotFound)
			}
		}
		return fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return nil
}

func (r *WorkerRepository) WorkerByID(ctx context.Context, workerID uuid.UUID) (live.Worker, error) {
	const op = "worker by id"
	const query = `
	SELECT id, node_id, host, grpc_port, container_id, exchange_account_id, status, last_error, created_at
	FROM workers
	WHERE id = $1`

	q := r.qp.Get(ctx)

	var w live.Worker
	err := q.QueryRow(ctx, query, workerID).Scan(
		&w.ID,
		&w.NodeID,
		&w.Host,
		&w.GRPCPort,
		&w.ContainerID,
		&w.ExchangeAccountID,
		&w.Status,
		&w.LastError,
		&w.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return live.Worker{}, fmt.Errorf("%s: %w", op, errorsx.ErrNotFound)
		}
		return live.Worker{}, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return w, nil
}

func (r *WorkerRepository) WorkerByAccountID(ctx context.Context, accountID uuid.UUID) (live.Worker, error) {
	const op = "worker by account id"
	const query = `
	SELECT id, node_id, host, grpc_port, container_id, exchange_account_id, status, last_error, created_at
	FROM workers
	WHERE exchange_account_id = $1;`

	q := r.qp.Get(ctx)

	var w live.Worker
	err := q.QueryRow(ctx, query, accountID).Scan(
		&w.ID,
		&w.NodeID,
		&w.Host,
		&w.GRPCPort,
		&w.ContainerID,
		&w.ExchangeAccountID,
		&w.Status,
		&w.LastError,
		&w.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return live.Worker{}, fmt.Errorf("%s: %w", op, errorsx.ErrNotFound)
		}
		return live.Worker{}, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return w, nil
}

func (r *WorkerRepository) WorkersByNodeID(ctx context.Context, nodeID uuid.UUID, statuses ...live.WorkerStatus) ([]live.Worker, error) {
	const op = "list workers by node id"

	q := r.qp.Get(ctx)

	var rows pgx.Rows
	var err error

	if len(statuses) == 0 {
		rows, err = q.Query(ctx, `
		SELECT id, node_id, host, grpc_port, container_id, exchange_account_id, status, last_error, created_at
		FROM workers
		WHERE node_id = $1`, nodeID)
	} else {
		rows, err = q.Query(ctx, `
		SELECT id, node_id, host, grpc_port, container_id, exchange_account_id, status, last_error, created_at
		FROM workers
		WHERE node_id = $1
		  AND status = ANY($2)`, nodeID, statuses)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	defer rows.Close()

	workers, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (live.Worker, error) {
		var w live.Worker
		if err := r.Scan(
			&w.ID,
			&w.NodeID,
			&w.Host,
			&w.GRPCPort,
			&w.ContainerID,
			&w.ExchangeAccountID,
			&w.Status,
			&w.LastError,
			&w.CreatedAt,
		); err != nil {
			return live.Worker{}, err
		}
		return w, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return workers, nil
}

func (r *WorkerRepository) DeleteWorkerByID(ctx context.Context, workerID uuid.UUID) error {
	const op = "delete worker by id"
	const query = `DELETE FROM workers WHERE id = $1;`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, workerID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() < 1 {
		return fmt.Errorf("%s: %w", op, errorsx.ErrNotFound)
	}

	return nil
}

func (r *WorkerRepository) Workers(ctx context.Context) ([]live.Worker, error) {
	const op = "workers"
	const query = `
	SELECT id, node_id, host, grpc_port, container_id, exchange_account_id, status, last_error, created_at
	FROM workers`

	q := r.qp.Get(ctx)

	rows, err := q.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	defer rows.Close()

	workers, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (live.Worker, error) {
		var w live.Worker
		if err := r.Scan(
			&w.ID,
			&w.NodeID,
			&w.Host,
			&w.GRPCPort,
			&w.ContainerID,
			&w.ExchangeAccountID,
			&w.Status,
			&w.LastError,
			&w.CreatedAt,
		); err != nil {
			return live.Worker{}, err
		}
		return w, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return workers, nil
}

func (r *WorkerRepository) WorkersByExchange(ctx context.Context, exchange string, statuses ...live.WorkerStatus) ([]live.Worker, error) {
	const op = "workers by exchange"

	q := r.qp.Get(ctx)

	var rows pgx.Rows
	var err error

	if len(statuses) == 0 {
		rows, err = q.Query(ctx, `
		SELECT w.id, w.node_id, w.host, w.grpc_port, w.container_id, w.exchange_account_id, w.status, w.last_error, w.created_at
		FROM workers w
		JOIN nodes n ON n.id = w.node_id
		WHERE n.exchange = $1`, exchange)
	} else {
		rows, err = q.Query(ctx, `
		SELECT w.id, w.node_id, w.host, w.grpc_port, w.container_id, w.exchange_account_id, w.status, w.last_error, w.created_at
		FROM workers w
		JOIN nodes n ON n.id = w.node_id
		WHERE n.exchange = $1
		  AND w.status = ANY($2)`, exchange, statuses)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	defer rows.Close()

	workers, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (live.Worker, error) {
		var w live.Worker
		if err := r.Scan(
			&w.ID,
			&w.NodeID,
			&w.Host,
			&w.GRPCPort,
			&w.ContainerID,
			&w.ExchangeAccountID,
			&w.Status,
			&w.LastError,
			&w.CreatedAt,
		); err != nil {
			return live.Worker{}, err
		}
		return w, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return workers, nil
}

func (r *WorkerRepository) UpdateWorkerDeploymentByID(ctx context.Context, workerID uuid.UUID, containerID string, grpcPort int) error {
	const op = "update worker deployment by id"
	const query = `UPDATE workers SET container_id = $2, grpc_port = $3 WHERE id = $1;`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, workerID, containerID, grpcPort)
	if err != nil {
		return fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, errorsx.ErrNotFound)
	}
	return nil
}

func (r *WorkerRepository) UpdateWorkerStatusByID(ctx context.Context, workerID uuid.UUID, status live.WorkerStatus, workerErr *string) error {
	const op = "update worker status by id"
	const query = `
	UPDATE workers
	SET status = $2,
	    last_error = $3
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, workerID, status, workerErr)
	if err != nil {
		return fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, errorsx.ErrNotFound)
	}
	return nil
}
