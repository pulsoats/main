package live

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/main/internal/domain/live"
	"github.com/pulsoats/main/internal/infrastructure/email/templates"
)

func (a *Application) CreateExchangeAccount(ctx context.Context, req live.CreateExchangeAccountRequest) error { //nolint
	const op = "create exchange account"

	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	account := live.ExchangeAccount{
		ID:          id,
		Exchange:    req.Exchange,
		Name:        req.Name,
		Credentials: &req.Credentials,
		ExpiresAt:   req.ExpiresAt,
	}

	return a.cfg.AccountRepo.CreateAccount(ctx, account)
}

func (a *Application) ExchangeAccounts(ctx context.Context) ([]live.ExchangeAccount, error) {
	return a.cfg.AccountRepo.Accounts(ctx)
}

func (a *Application) AccountByID(ctx context.Context, accountID uuid.UUID) (live.ExchangeAccount, error) {
	return a.cfg.AccountRepo.AccountByID(ctx, accountID)
}

func (a *Application) UpdateAccountName(ctx context.Context, accountID uuid.UUID, newName string) error {
	return a.cfg.AccountRepo.UpdateName(ctx, accountID, newName)
}

func (a *Application) StartExpiringAccountsChecker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.checkExpiringAccounts(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (a *Application) checkExpiringAccounts(ctx context.Context) {
	const op = "check expiring accounts"
	accounts, err := a.cfg.AccountRepo.Accounts(ctx)
	if err != nil {
		a.cfg.Logger.Error("failed to fetch accounts", "op", op, "error", err)
	}

	for _, acc := range accounts {
		now := time.Now()
		if acc.ExpiresAt.After(now) && acc.ExpiresAt.Before(now.AddDate(0, 0, 7)) {
			sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err := a.cfg.EmailSender.Send(sendCtx, templates.AccountExpiryReminder(acc.Email, a.cfg.AppName, acc.Name, acc.Exchange, int(time.Until(acc.ExpiresAt).Hours()/24)))
			if err != nil {
				a.cfg.Logger.Error("failed to send expiry reminder", "op", op, "error", err, "account_id", acc.ID)
			}
			cancel()
		}

		if acc.ExpiresAt.Before(now) {
			sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err := a.cfg.EmailSender.Send(sendCtx, templates.AccountExpired(acc.Email, a.cfg.AppName, acc.Name, acc.Exchange))
			if err != nil {
				a.cfg.Logger.Error("failed to send expiry notification", "op", op, "error", err, "account_id", acc.ID)
			}
			cancel()
		}
	}
}
