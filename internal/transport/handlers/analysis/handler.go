package analysis

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pulsoats/core/domain/market"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/domain/analysis"
	"github.com/pulsoats/main/internal/transport/errorx"
	"github.com/pulsoats/main/internal/transport/middleware"
)

type Handler struct {
	app app
}

func NewHandler(app app) *Handler {
	return &Handler{app: app}
}

func (h *Handler) StartRun(c *gin.Context) {
	var req startRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: %s", errorsx.ErrInvalidArgument, err.Error()))
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		errorx.RespondError(c, errorsx.ErrUnauthorized)
		return
	}

	interval, ok := market.ParseInterval(req.Interval)
	if !ok {
		errorx.RespondError(c, fmt.Errorf("%w: interval %s", errorsx.ErrNotFound, req.Interval))
		return
	}

	from, err := time.Parse(time.RFC3339, req.From)
	if err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: fromTime", errorsx.ErrInvalidArgument))
		return
	}

	to, err := time.Parse(time.RFC3339, req.To)
	if err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: toTime", errorsx.ErrInvalidArgument))
		return
	}

	startRunReq := analysis.StartRunRequest{
		UserID:    userID.String(),
		Market:    mapToMarketSpec(req.Market),
		Interval:  interval,
		From:      from,
		To:        to,
		PriceType: market.PriceType(req.PriceType),
		Detector:  mapToDetectorConfig(req.Detector),
		Fees:      mapToFees(req.Fees),
	}

	runID, err := h.app.StartRun(c.Request.Context(), startRunReq)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, startRunResponse{RunID: runID})
}

func (h *Handler) RunStatus(c *gin.Context) {
	runID := c.Param("run_id")
	if strings.TrimSpace(runID) == "" {
		errorx.RespondError(c, fmt.Errorf("%w: runId", errorsx.ErrRequired))
		return
	}

	status, err := h.app.RunStatus(c.Request.Context(), runID)
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
		errorx.RespondError(c, fmt.Errorf("%w: runId", errorsx.ErrRequired))
		return
	}

	meta, err := h.app.RunMeta(c.Request.Context(), runID)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, mapToRunMetaResponse(meta))
}

func (h *Handler) RunResult(c *gin.Context) {
	runID := c.Param("run_id")
	if strings.TrimSpace(runID) == "" {
		errorx.RespondError(c, fmt.Errorf("%w: runId", errorsx.ErrRequired))
		return
	}

	archive, err := h.app.RunResult(c.Request.Context(), runID)
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

func (h *Handler) ListRuns(c *gin.Context) {
	limit := 20
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			errorx.RespondError(c, fmt.Errorf("%w: limit", errorsx.ErrInvalidArgument))
			return
		}
		limit = parsed
	}

	var beforeID *int64
	if rawBefore := strings.TrimSpace(c.Query("beforeId")); rawBefore != "" {
		id, err := strconv.ParseInt(rawBefore, 10, 64)
		if err != nil {
			errorx.RespondError(c, fmt.Errorf("%w: before_id", errorsx.ErrInvalidArgument))
			return
		}
		beforeID = &id
	}

	page, err := h.app.ListRunsPaged(c.Request.Context(), limit, beforeID)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, mapRunsPageToResponse(page))
}
