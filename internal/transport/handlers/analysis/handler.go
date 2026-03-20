package analysis

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/lib/errorsx"
	"github.com/pulsoats/main/internal/domain/analysis"
	"github.com/pulsoats/main/internal/transport/errorx"
	"github.com/pulsoats/main/internal/transport/middleware"
)

type Handler struct {
	service analysis.Service
}

func NewHandler(service analysis.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) StartRun(c *gin.Context) {
	var req startRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: %s", derrors.ErrInvalidArgument, err.Error()))
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		errorx.RespondError(c, derrors.ErrUnauthorized)
		return
	}

	interval, ok := market.ParseInterval(req.Interval)
	if !ok {
		errorx.RespondError(c, fmt.Errorf("%w: interval %s", derrors.ErrNotFound, req.Interval))
		return
	}

	from, err := time.Parse(time.RFC3339, req.From)
	if err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: from_time", derrors.ErrInvalidArgument))
		return
	}

	to, err := time.Parse(time.RFC3339, req.To)
	if err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: to_time", derrors.ErrInvalidArgument))
		return
	}

	startRunReq := analysis.StartRunRequest{
		UserID:    strconv.FormatInt(userID, 10),
		Market:    mapToMarketSpec(req.Market),
		Interval:  interval,
		From:      from,
		To:        to,
		PriceType: market.PriceType(req.PriceType),
		Detector:  mapToDetectorConfig(req.Detector),
		Fees:      mapToFees(req.Fees),
	}

	runID, err := h.service.StartRun(c.Request.Context(), startRunReq)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, startRunResponse{RunID: runID})
}

func (h *Handler) RunStatus(c *gin.Context) {
	runID := c.Param("run_id")
	if strings.TrimSpace(runID) == "" {
		errorx.RespondError(c, fmt.Errorf("%w: run_id", derrors.ErrRequired))
		return
	}

	status, err := h.service.RunStatus(c.Request.Context(), runID)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, runStatusResponse{
		Status:  analysis.StatusName(status.Code),
		Message: status.Message,
	})
}

func (h *Handler) RunMeta(c *gin.Context) {
	runID := c.Param("run_id")
	if strings.TrimSpace(runID) == "" {
		errorx.RespondError(c, fmt.Errorf("%w: run_id", derrors.ErrRequired))
		return
	}

	meta, err := h.service.RunMeta(c.Request.Context(), runID)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, mapToRunMetaResponse(meta))
}

func (h *Handler) RunResult(c *gin.Context) {
	runID := c.Param("run_id")
	if strings.TrimSpace(runID) == "" {
		errorx.RespondError(c, fmt.Errorf("%w: run_id", derrors.ErrRequired))
		return
	}

	archive, err := h.service.RunResult(c.Request.Context(), runID)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}
	defer archive.Content.Close()

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, archive.Filename))
	if _, err := io.Copy(c.Writer, archive.Content); err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: %v", errorsx.ErrInternal, err))
		return
	}
}
