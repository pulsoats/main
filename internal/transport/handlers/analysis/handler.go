package analysis

import (
	"encoding/json"
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
	filter := c.DefaultQuery("filter", "mine")

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

	page, err := h.app.ListRunsPaged(c.Request.Context(), limit, beforeID, filter)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, mapRunsPageToResponse(page))
}

func (h *Handler) ShareRun(c *gin.Context) {
	runID := c.Param("run_id")
	if strings.TrimSpace(runID) == "" {
		errorx.RespondError(c, fmt.Errorf("%w: run_id", errorsx.ErrRequired))
		return
	}

	if err := h.app.ShareRun(c.Request.Context(), runID); err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"shared": true})
}

func (h *Handler) StreamRunMeta(c *gin.Context) {
	runID := c.Param("run_id")
	if strings.TrimSpace(runID) == "" {
		errorx.RespondError(c, fmt.Errorf("%w: run_id", errorsx.ErrRequired))
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		errorx.RespondError(c, fmt.Errorf("%w: streaming not supported", errorsx.ErrInternal))
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	ctx := c.Request.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			meta, err := h.app.RunMeta(ctx, runID)
			if err != nil {
				fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", err.Error())
				flusher.Flush()
				return
			}

			data, _ := json.Marshal(mapToRunMetaResponse(meta))
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			flusher.Flush()

			if meta.Status.Code == analysis.StatusDone || meta.Status.Code == analysis.StatusFailed {
				fmt.Fprintf(c.Writer, "event: done\n\n")
				flusher.Flush()
				return
			}
		}
	}
}
