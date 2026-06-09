package live

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/exchange"
)

type ExchangeAccount struct {
	ID          uuid.UUID
	Exchange    string
	Name        string
	Email       string
	Credentials *exchange.Credentials
	ExpiresAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ExchangeAccountRepository interface {
	CreateAccount(ctx context.Context, account ExchangeAccount) error
	AccountByID(ctx context.Context, id uuid.UUID) (ExchangeAccount, error)
	AccountByIDWithCredentials(ctx context.Context, id uuid.UUID) (ExchangeAccount, error)
	Accounts(ctx context.Context) ([]ExchangeAccount, error)
	UpdateName(ctx context.Context, id uuid.UUID, name string) error
	UpdateCredentials(ctx context.Context, id uuid.UUID, creds exchange.Credentials, expiresAt time.Time) error
	DeleteAccountByID(ctx context.Context, id uuid.UUID) error
}

type CreateExchangeAccountRequest struct {
	Exchange    string
	Name        string
	Credentials exchange.Credentials
	ExpiresAt   time.Time
}
