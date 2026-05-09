package market

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pulsoats/core/errorsx"
	domainmarket "github.com/pulsoats/main/internal/domain/market"
	"github.com/pulsoats/main/internal/transport/errhttp"
)

type app interface {
	ListSymbols(ctx context.Context, exchange string, category string) ([]string, error)
	Suggest(ctx context.Context, exchange, query string, limit int) ([]domainmarket.Suggestion, error)
}

type Handler struct {
	app app
}

func NewHandler(app app) *Handler {
	return &Handler{app: app}
}

func (h *Handler) ListSymbols(c *gin.Context) {
	ex := strings.TrimSpace(c.Query("exchange"))
	category := strings.TrimSpace(c.Query("category"))

	if ex == "" || category == "" {
		errhttp.RespondError(c, fmt.Errorf("%w: exchange and category are required", errorsx.ErrInvalidArgument))
		return
	}

	symbols, err := h.app.ListSymbols(c.Request.Context(), ex, category)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"symbols": symbols})
}

func (h *Handler) Suggest(c *gin.Context) {
	ex := strings.TrimSpace(c.Query("exchange"))
	query := strings.TrimSpace(c.Query("query"))

	if ex == "" {
		errhttp.RespondError(c, fmt.Errorf("%w: exchange is required", errorsx.ErrInvalidArgument))
		return
	}

	limit := 20
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			errhttp.RespondError(c, fmt.Errorf("limit: %w", errorsx.ErrInvalidArgument))
			return
		}
		limit = parsed
	}

	suggestions, err := h.app.Suggest(c.Request.Context(), ex, query, limit)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, suggestionsToResponse(suggestions))
}
