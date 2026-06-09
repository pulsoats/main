package live

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	"github.com/pulsoats/main/internal/domain/live"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres"
)

type encryptor interface {
	Encrypt(plaintext string) ([]byte, error)
	Decrypt(ciphertext []byte) (string, error)
}

type ExchangeAccountRepository struct {
	qp  postgres.QuerierProvider
	enc encryptor
}

func NewPostgresExchangeAccountRepository(qp postgres.QuerierProvider, enc encryptor) *ExchangeAccountRepository {
	return &ExchangeAccountRepository{qp: qp, enc: enc}
}

func (r *ExchangeAccountRepository) CreateAccount(ctx context.Context, account live.ExchangeAccount) error {
	const query = `
	INSERT INTO exchange_accounts (id, exchange, name, email, api_key_enc, api_secret_enc, passphrase_enc, expires_at, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW());`

	q := r.qp.Get(ctx)

	var apiKeyEnc, apiSecretEnc, passphraseEnc []byte
	var err error
	var expiresAt interface{}

	if account.Credentials != nil {
		if account.Credentials.APIKey != "" {
			apiKeyEnc, err = r.enc.Encrypt(account.Credentials.APIKey)
			if err != nil {
				return fmt.Errorf("create account: api key: %w", err)
			}
		}
		if account.Credentials.APISecret != "" {
			apiSecretEnc, err = r.enc.Encrypt(account.Credentials.APISecret)
			if err != nil {
				return fmt.Errorf("create account: api secret: %w", err)
			}
		}
		if account.Credentials.Passphrase != "" {
			passphraseEnc, err = r.enc.Encrypt(account.Credentials.Passphrase)
			if err != nil {
				return fmt.Errorf("create account: passphrase: %w", err)
			}
		}
		if !account.ExpiresAt.IsZero() {
			expiresAt = account.ExpiresAt
		}
	}

	_, err = q.Exec(ctx, query,
		account.ID,
		account.Exchange,
		account.Name,
		account.Email,
		apiKeyEnc,
		apiSecretEnc,
		passphraseEnc,
		expiresAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("create account: %w", errorsx.ErrAlreadyExists)
		}
		return fmt.Errorf("create account: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return nil
}

func (r *ExchangeAccountRepository) AccountByID(ctx context.Context, id uuid.UUID) (live.ExchangeAccount, error) {
	const query = `
	SELECT id, exchange, name, email, expires_at, created_at, updated_at
	FROM exchange_accounts
	WHERE id = $1;`

	q := r.qp.Get(ctx)

	var a live.ExchangeAccount
	var expiresAt *time.Time
	err := q.QueryRow(ctx, query, id).Scan(
		&a.ID,
		&a.Exchange,
		&a.Name,
		&a.Email,
		&expiresAt,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return live.ExchangeAccount{}, fmt.Errorf("account by id: %w", errorsx.ErrNotFound)
		}
		return live.ExchangeAccount{}, fmt.Errorf("account by id: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if expiresAt != nil {
		a.ExpiresAt = *expiresAt
	}

	return a, nil
}

func (r *ExchangeAccountRepository) AccountByIDWithCredentials(ctx context.Context, id uuid.UUID) (live.ExchangeAccount, error) {
	const query = `
	SELECT id, exchange, name, email, api_key_enc, api_secret_enc, passphrase_enc, expires_at, created_at, updated_at
	FROM exchange_accounts
	WHERE id = $1;`

	q := r.qp.Get(ctx)

	var a live.ExchangeAccount
	var apiKeyEnc, apiSecretEnc, passphraseEnc []byte
	var expiresAt *time.Time
	err := q.QueryRow(ctx, query, id).Scan(
		&a.ID,
		&a.Exchange,
		&a.Name,
		&a.Email,
		&apiKeyEnc,
		&apiSecretEnc,
		&passphraseEnc,
		&expiresAt,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return live.ExchangeAccount{}, fmt.Errorf("account by id with credentials: %w", errorsx.ErrNotFound)
		}
		return live.ExchangeAccount{}, fmt.Errorf("account by id with credentials: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if expiresAt != nil {
		a.ExpiresAt = *expiresAt
	}

	if apiKeyEnc != nil || apiSecretEnc != nil || passphraseEnc != nil {
		c := &exchange.Credentials{}
		if apiKeyEnc != nil {
			c.APIKey, err = r.enc.Decrypt(apiKeyEnc)
			if err != nil {
				return live.ExchangeAccount{}, fmt.Errorf("account by id with credentials: api key: %w", err)
			}
		}
		if apiSecretEnc != nil {
			c.APISecret, err = r.enc.Decrypt(apiSecretEnc)
			if err != nil {
				return live.ExchangeAccount{}, fmt.Errorf("account by id with credentials: api secret: %w", err)
			}
		}
		if passphraseEnc != nil {
			c.Passphrase, err = r.enc.Decrypt(passphraseEnc)
			if err != nil {
				return live.ExchangeAccount{}, fmt.Errorf("account by id with credentials: passphrase: %w", err)
			}
		}
		a.Credentials = c
	}

	return a, nil
}

func (r *ExchangeAccountRepository) Accounts(ctx context.Context) ([]live.ExchangeAccount, error) {
	const query = `
	SELECT id, exchange, name, email, expires_at, created_at, updated_at
	FROM exchange_accounts
	ORDER BY created_at DESC;`

	q := r.qp.Get(ctx)

	rows, err := q.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", errors.Join(errorsx.ErrInternal, err))
	}
	defer rows.Close()

	accounts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (live.ExchangeAccount, error) {
		var a live.ExchangeAccount
		var expiresAt *time.Time
		if err := row.Scan(
			&a.ID,
			&a.Exchange,
			&a.Name,
			&a.Email,
			&expiresAt,
			&a.CreatedAt,
			&a.UpdatedAt,
		); err != nil {
			return live.ExchangeAccount{}, fmt.Errorf("list accounts: %w", errors.Join(errorsx.ErrInternal, err))
		}
		if expiresAt != nil {
			a.ExpiresAt = *expiresAt
		}
		return a, nil
	})
	if err != nil {
		return nil, err
	}

	return accounts, nil
}

func (r *ExchangeAccountRepository) UpdateName(ctx context.Context, id uuid.UUID, name string) error {
	const query = `
	UPDATE exchange_accounts
	SET name       = $2,
	    updated_at = NOW()
	WHERE id = $1;`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, id, name)
	if err != nil {
		return fmt.Errorf("update name: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update name: %w", errorsx.ErrNotFound)
	}

	return nil
}

func (r *ExchangeAccountRepository) UpdateCredentials(ctx context.Context, id uuid.UUID, creds exchange.Credentials, expiresAt time.Time) error {
	const query = `
	UPDATE exchange_accounts
	SET api_key_enc    = $2,
	    api_secret_enc = $3,
	    passphrase_enc = $4,
	    expires_at     = $5,
	    updated_at     = NOW()
	WHERE id = $1;`

	q := r.qp.Get(ctx)

	apiKeyEnc, err := r.enc.Encrypt(creds.APIKey)
	if err != nil {
		return fmt.Errorf("update credentials: api key: %w", err)
	}

	var apiSecretEnc []byte
	if creds.APISecret != "" {
		apiSecretEnc, err = r.enc.Encrypt(creds.APISecret)
		if err != nil {
			return fmt.Errorf("update credentials: api secret: %w", err)
		}
	}

	var passphraseEnc []byte
	if creds.Passphrase != "" {
		passphraseEnc, err = r.enc.Encrypt(creds.Passphrase)
		if err != nil {
			return fmt.Errorf("update credentials: passphrase: %w", err)
		}
	}

	var expiresAtVal interface{}
	if !expiresAt.IsZero() {
		expiresAtVal = expiresAt
	}

	tag, err := q.Exec(ctx, query, id, apiKeyEnc, apiSecretEnc, passphraseEnc, expiresAtVal)
	if err != nil {
		return fmt.Errorf("update credentials: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update credentials: %w", errorsx.ErrNotFound)
	}

	return nil
}

func (r *ExchangeAccountRepository) DeleteAccountByID(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM exchange_accounts WHERE id = $1;`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete account by id: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delete account by id: %w", errorsx.ErrNotFound)
	}

	return nil
}
