package detectors

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pulsoats/core/domain/detect"
)

type app interface {
	ListMetas() []detect.DetectorMeta
}

type Handler struct {
	app app
}

func NewHandler(app app) *Handler {
	return &Handler{app: app}
}

func (h *Handler) ListMetas(c *gin.Context) {
	metas := h.app.ListMetas()
	resp := mapMetasToResponseSlice(metas)

	c.JSON(http.StatusOK, resp)
}
