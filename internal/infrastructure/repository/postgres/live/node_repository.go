package live

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/domain/live"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres"
)

type NodeRepository struct {
	qp postgres.QuerierProvider
}

func NewPostgresNodeRepository(qp postgres.QuerierProvider) *NodeRepository {
	return &NodeRepository{qp: qp}
}

func (r *NodeRepository) CreateNode(ctx context.Context, s *live.Node) error {
	const op = "create node"
	const query = `
	INSERT INTO nodes (id, exchange, host, docker_port, region, dsn, max_workers, status)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING created_at;`

	q := r.qp.Get(ctx)

	err := q.QueryRow(ctx, query,
		s.ID,
		s.Exchange,
		s.Host,
		s.DockerPort,
		s.Region,
		s.DSN,
		s.MaxWorkers,
		s.Status,
	).Scan(&s.CreatedAt)
	if err != nil {
		return fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return nil
}

func (r *NodeRepository) NodeByID(ctx context.Context, id uuid.UUID) (live.Node, error) {
	const op = "node by id"
	const query = `
	SELECT n.id, n.exchange, n.host, n.docker_port, n.region, n.dsn, n.max_workers,
	       n.status, n.last_error, n.created_at,
	       COUNT(w.id) AS workers_count
	FROM nodes n
	LEFT JOIN workers w ON w.node_id = n.id
	WHERE n.id = $1
	GROUP BY n.id;`

	q := r.qp.Get(ctx)

	var s live.Node
	err := q.QueryRow(ctx, query, id).Scan(
		&s.ID,
		&s.Exchange,
		&s.Host,
		&s.DockerPort,
		&s.Region,
		&s.DSN,
		&s.MaxWorkers,
		&s.Status,
		&s.LastError,
		&s.CreatedAt,
		&s.WorkersCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return live.Node{}, fmt.Errorf("%s: %w", op, errorsx.ErrNotFound)
		}
		return live.Node{}, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return s, nil
}

func (r *NodeRepository) Nodes(ctx context.Context) ([]live.Node, error) {
	const op = "list nodes"
	const query = `
	SELECT n.id, n.exchange, n.host, n.docker_port, n.region, n.dsn, n.max_workers,
	       n.status, n.last_error, n.created_at,
	       COUNT(w.id) AS workers_count
	FROM nodes n
	LEFT JOIN workers w ON w.node_id = n.id
	GROUP BY n.id;`

	q := r.qp.Get(ctx)

	rows, err := q.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	defer rows.Close()

	nodes, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (live.Node, error) {
		var s live.Node
		if err := row.Scan(
			&s.ID,
			&s.Exchange,
			&s.Host,
			&s.DockerPort,
			&s.Region,
			&s.DSN,
			&s.MaxWorkers,
			&s.Status,
			&s.LastError,
			&s.CreatedAt,
			&s.WorkersCount,
		); err != nil {
			return live.Node{}, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
		}
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	return nodes, nil
}

func (r *NodeRepository) NodesByExchange(ctx context.Context, exchange string) ([]live.Node, error) {
	const op = "list nodes by exchange"
	const query = `
	SELECT n.id, n.exchange, n.host, n.docker_port, n.region, n.dsn, n.max_workers,
	       n.status, n.last_error, n.created_at,
	       COUNT(w.id) AS workers_count
	FROM nodes n
	LEFT JOIN workers w ON w.node_id = n.id
	WHERE n.exchange = $1
	GROUP BY n.id
	ORDER BY workers_count ASC;`

	q := r.qp.Get(ctx)

	rows, err := q.Query(ctx, query, exchange)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	defer rows.Close()

	nodes, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (live.Node, error) {
		var s live.Node
		if err := row.Scan(
			&s.ID,
			&s.Exchange,
			&s.Host,
			&s.DockerPort,
			&s.Region,
			&s.DSN,
			&s.MaxWorkers,
			&s.Status,
			&s.LastError,
			&s.CreatedAt,
			&s.WorkersCount,
		); err != nil {
			return live.Node{}, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
		}
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	return nodes, nil
}

func (r *NodeRepository) LeastLoadedNodeByExchange(ctx context.Context, exchange string) (live.Node, error) {
	const op = "least loaded node by exchange"
	const query = `
	SELECT n.id, n.exchange, n.host, n.docker_port, n.region, n.dsn, n.max_workers,
	       n.status, n.last_error, n.created_at,
	       COUNT(w.id) AS workers_count
	FROM nodes n
	LEFT JOIN workers w ON w.node_id = n.id
	WHERE n.exchange = $1
	  AND n.status = 'active'
	GROUP BY n.id
	HAVING COUNT(w.id) < n.max_workers
	ORDER BY workers_count ASC
	LIMIT 1;`

	q := r.qp.Get(ctx)

	var n live.Node
	err := q.QueryRow(ctx, query, exchange).Scan(
		&n.ID,
		&n.Exchange,
		&n.Host,
		&n.DockerPort,
		&n.Region,
		&n.DSN,
		&n.MaxWorkers,
		&n.Status,
		&n.LastError,
		&n.CreatedAt,
		&n.WorkersCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return live.Node{}, fmt.Errorf("%s: %w", op, errorsx.ErrNotFound)
		}
		return live.Node{}, fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return n, nil
}

func (r *NodeRepository) UpdateNodeStatusByID(ctx context.Context, nodeID uuid.UUID, status live.NodeStatus, nodeErr *string) error {
	const op = "update node status by id"
	const query = `
	UPDATE nodes
	SET status     = $2,
	    last_error = $3
	WHERE id = $1;`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, nodeID, status, nodeErr)
	if err != nil {
		return fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, errorsx.ErrNotFound)
	}

	return nil
}

func (r *NodeRepository) UpdateNodeDSNByID(ctx context.Context, nodeID uuid.UUID, dsn string) error {
	const op = "update node dsn by id"
	const query = `
	UPDATE nodes
	SET dsn = $2
	WHERE id = $1;`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, nodeID, dsn)
	if err != nil {
		return fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, errorsx.ErrNotFound)
	}

	return nil
}

func (r *NodeRepository) DeleteNodeByID(ctx context.Context, nodeID uuid.UUID) error {
	const op = "delete node by id"
	const query = `DELETE FROM nodes WHERE id = $1;`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, nodeID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, errorsx.ErrNotFound)
	}

	return nil
}
