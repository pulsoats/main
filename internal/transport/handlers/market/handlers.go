package market

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pulsoats/core/domain/exchange"
	coremarket "github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/transport/errorx"
)

type app interface {
	ListExchangeMetas() []exchange.Meta
	ListSymbols(ctx context.Context, exchange string, category coremarket.Category) ([]string, error)
}

type Handler struct {
	app app
}

func NewHandler(app app) *Handler {
	return &Handler{app: app}
}

func (h *Handler) ListExchangeMetas(c *gin.Context) {
	metas := h.app.ListExchangeMetas()

	resp := mapExchangeMetasToResponseSlice(metas)
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListSymbols(c *gin.Context) {
	ex := strings.TrimSpace(c.Query("exchange"))
	category := strings.TrimSpace(c.Query("category"))

	if ex == "" || category == "" {
		errorx.RespondError(c, fmt.Errorf("%w: exchange and category", errorsx.ErrInvalidArgument))
		return
	}

	symbols, err := h.app.ListSymbols(c.Request.Context(), ex, coremarket.Category(category))
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, symbols)
}
