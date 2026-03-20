package detectors

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pulsoats/main/internal/domain/detectors"
)

type Handler struct {
	service detectors.Service
}

func NewHandler(service detectors.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListMetas(c *gin.Context) {
	metas := h.service.ListMetas()
	resp := mapMetasToResponse(metas)

	c.JSON(http.StatusOK, resp)
}
