package live

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	domainlive "github.com/pulsoats/main/internal/domain/live"
	"github.com/pulsoats/main/internal/transport/errhttp"
)

func (h *Handler) CreateAccount(c *gin.Context) {
	var req createAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errhttp.RespondError(c, errorsx.ErrInvalidArgument)
		return
	}

	if err := h.app.CreateExchangeAccount(c.Request.Context(), domainlive.CreateExchangeAccountRequest{
		Exchange: req.Exchange,
		Name:     req.Name,
		Credentials: exchange.Credentials{
			APIKey:     req.Credentials.APIKey,
			APISecret:  req.Credentials.APISecret,
			Passphrase: req.Credentials.Passphrase,
		},
		ExpiresAt: req.ExpiresAt,
	}); err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.Status(http.StatusCreated)
}

func (h *Handler) Accounts(c *gin.Context) {
	accounts, err := h.app.ExchangeAccounts(c.Request.Context())
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, accountsToResponse(accounts))
}

func (h *Handler) AccountByID(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	account, err := h.app.AccountByID(c.Request.Context(), accountID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, accountToResponse(account))
}

func (h *Handler) UpdateAccountName(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errhttp.RespondError(c, errorsx.ErrInvalidArgument)
		return
	}

	if err := h.app.UpdateAccountName(c.Request.Context(), accountID, req.Name); err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
