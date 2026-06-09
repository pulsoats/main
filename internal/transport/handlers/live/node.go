package live

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/application/live"
	domainlive "github.com/pulsoats/main/internal/domain/live"
	"github.com/pulsoats/main/internal/transport/errhttp"
	"github.com/pulsoats/main/internal/transport/middleware"
)

func (h *Handler) CreateNode(c *gin.Context) {
	var req createNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errhttp.RespondError(c, errorsx.ErrInvalidArgument)
		return
	}

	node, err := h.app.CreateNode(c.Request.Context(), domainlive.AddNodeRequest{
		Exchange:   req.Exchange,
		Host:       req.Host,
		DockerPort: req.DockerPort,
		Region:     req.Region,
		MaxWorkers: req.MaxWorkers,
		DBUser:     req.DBUser,
		DBPassword: req.DBPassword,
	})
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, nodeToResponse(node))
}

func (h *Handler) DisableNode(c *gin.Context) {
	callerID, ok := middleware.GetUserID(c)
	if !ok {
		errhttp.RespondError(c, errorsx.ErrForbidden)
		return
	}

	nodeID, err := uuid.Parse(c.Param("node_id"))
	if err != nil {
		errhttp.RespondError(c, errorsx.ErrInvalidArgument)
		return
	}

	if err := h.app.DisableNode(c.Request.Context(), nodeID, callerID); err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.Status(http.StatusAccepted)
}

func (h *Handler) EnableNode(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("node_id"))
	if err != nil {
		errhttp.RespondError(c, errorsx.ErrInvalidArgument)
		return
	}

	if err := h.app.EnableNode(c.Request.Context(), nodeID); err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) NodeByID(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("node_id"))
	if err != nil {
		errhttp.RespondError(c, errorsx.ErrInvalidArgument)
		return
	}

	node, err := h.app.NodeByID(c.Request.Context(), nodeID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, nodeToResponse(node))
}

func (h *Handler) Nodes(c *gin.Context) {
	var f live.NodesFilter
	if exchange := c.Query("exchange"); exchange != "" {
		f.Exchange = &exchange
	}

	nodes, err := h.app.Nodes(c.Request.Context(), f)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, nodesToResponse(nodes))
}

func (h *Handler) DeleteNode(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("node_id"))
	if err != nil {
		errhttp.RespondError(c, errorsx.ErrInvalidArgument)
		return
	}

	if err := h.app.DeleteNode(c.Request.Context(), nodeID); err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
