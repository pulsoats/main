package system

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pulsoats/core/errorsx"
	coresystem "github.com/pulsoats/core/system"
	domainsystem "github.com/pulsoats/main/internal/domain/system"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres"
)

type Repository struct {
	qp postgres.QuerierProvider
}

func NewPostgresRepository(qp postgres.QuerierProvider) *Repository {
	return &Repository{qp: qp}
}

func (r *Repository) CreateService(ctx context.Context, s *domainsystem.Service) error {
	const query = `
	INSERT INTO services (id, kind, addr, name, exchange, account, version, last_seen_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	ON CONFLICT (exchange, account) DO UPDATE
	    SET id           = EXCLUDED.id,
	        addr         = EXCLUDED.addr,
	        version      = EXCLUDED.version,
	        last_seen_at = NOW()
	RETURNING created_at, last_seen_at;
	`

	q := r.qp.Get(ctx)

	err := q.QueryRow(ctx, query,
		s.ID,
		string(s.Kind),
		s.Addr,
		s.Name,
		s.Exchange,
		s.Account,
		s.Version,
	).Scan(&s.CreatedAt, &s.LastSeenAt)
	if err != nil {
		return fmt.Errorf("upsert service: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return nil
}

func (r *Repository) ServiceByID(ctx context.Context, id uuid.UUID) (domainsystem.Service, error) {
	const query = `
	SELECT id, kind, addr, name, exchange, account, version, last_seen_at, created_at
	FROM services
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)

	var s domainsystem.Service
	var kind string
	err := q.QueryRow(ctx, query, id).Scan(
		&s.ID,
		&kind,
		&s.Addr,
		&s.Name,
		&s.Exchange,
		&s.Account,
		&s.Version,
		&s.LastSeenAt,
		&s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainsystem.Service{}, fmt.Errorf("service by id: %w", errorsx.ErrNotFound)
		}
		return domainsystem.Service{}, fmt.Errorf("service by id: %w", errors.Join(errorsx.ErrInternal, err))
	}

	s.Kind = parseKind(kind)
	return s, nil
}

func (r *Repository) ListServices(ctx context.Context) ([]domainsystem.Service, error) {
	const query = `
	SELECT id, kind, addr, name, exchange, account, version, last_seen_at, created_at
	FROM services
	ORDER BY created_at;
	`

	q := r.qp.Get(ctx)

	rows, err := q.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", errors.Join(errorsx.ErrInternal, err))
	}
	defer rows.Close()

	services, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domainsystem.Service, error) {
		var s domainsystem.Service
		var kind string
		if err := row.Scan(
			&s.ID,
			&kind,
			&s.Addr,
			&s.Name,
			&s.Exchange,
			&s.Account,
			&s.Version,
			&s.LastSeenAt,
			&s.CreatedAt,
		); err != nil {
			return domainsystem.Service{}, fmt.Errorf("list services: %w", errors.Join(errorsx.ErrInternal, err))
		}
		s.Kind = parseKind(kind)
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	return services, nil
}

func parseKind(s string) coresystem.ServiceKind {
	return coresystem.ServiceKind(s)
}

func (r *Repository) DeleteService(ctx context.Context, exchange, account string) error {
	const query = `DELETE FROM services WHERE exchange = $1 AND account = $2;`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, exchange, account)
	if err != nil {
		return fmt.Errorf("delete service: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delete service: %w", errorsx.ErrNotFound)
	}

	return nil
}
