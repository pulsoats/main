package market

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pulsoats/core/domain/derrors"
	coremarket "github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/main/internal/domain/market"
	"github.com/pulsoats/main/internal/transport/errorx"
)

type Handler struct {
	service market.Service
}

func NewHandler(service market.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListExchangeMetas(c *gin.Context) {
	metas := h.service.ListExchangeMetas()

	resp := mapExchangeMetasToResponse(metas)
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListSymbols(c *gin.Context) {
	exchange := strings.TrimSpace(c.Query("exchange"))
	category := strings.TrimSpace(c.Query("category"))

	if exchange == "" || category == "" {
		errorx.RespondError(c, fmt.Errorf("%w: exchange/category", derrors.ErrRequired))
		return
	}

	symbols, err := h.service.ListSymbols(c.Request.Context(), exchange, coremarket.Category(category))
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, listSymbolsResponse{Symbols: symbols})
}
