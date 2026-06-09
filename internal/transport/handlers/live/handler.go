package live

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/application/live"
	"github.com/pulsoats/main/internal/transport/errhttp"
)

type Handler struct {
	app *live.Application
}

func NewHandler(app *live.Application) (*Handler, error) {
	if app == nil {
		return nil, fmt.Errorf("live handler: app: %w", errorsx.ErrRequired)
	}
	return &Handler{app: app}, nil
}

func (h *Handler) resolveAccountID(c *gin.Context) (uuid.UUID, bool) {
	accountID, err := uuid.Parse(c.Param("account_id"))
	if err != nil {
		errhttp.RespondError(c, fmt.Errorf("account_id: %w", errorsx.ErrInvalidArgument))
		return uuid.Nil, false
	}
	return accountID, true
}
