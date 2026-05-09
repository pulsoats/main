package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	corerun "github.com/pulsoats/core/run"
	coresystem "github.com/pulsoats/core/system"
	appanalysis "github.com/pulsoats/main/internal/application/analysis"
	"github.com/pulsoats/main/internal/domain/analysis"
	"github.com/pulsoats/main/internal/transport/errhttp"
	"github.com/pulsoats/main/internal/transport/handlers/core"
	"github.com/pulsoats/main/internal/transport/middleware"
)

type app interface {
	NewRun(ctx context.Context, callerID uuid.UUID, req analysis.NewRunRequest) (analysis.Run, error)
	RunByID(ctx context.Context, runID uuid.UUID) (analysis.Run, error)
	StreamRunArchive(ctx context.Context, runID uuid.UUID, dst io.Writer) error
	ShareRun(ctx context.Context, callerID, runID uuid.UUID) error
	DeleteRun(ctx context.Context, callerID, runID uuid.UUID) error
	ListRunsPaged(ctx context.Context, callerID uuid.UUID, req analysis.ListRunsPagedRequest) (analysis.ListRunsPagedResponse, error)

	ListAvailableExchanges(ctx context.Context) ([]exchange.Meta, error)
	ListAvailableDetectors(ctx context.Context) ([]detect.DetectorMeta, error)

	Info(ctx context.Context) (coresystem.ServiceInfo, error)
	Metrics(ctx context.Context) (coresystem.ServiceMetrics, error)
}

type Handler struct {
	app *appanalysis.Application
}

func NewHandler(app *appanalysis.Application) (*Handler, error) {
	if app == nil {
		return nil, fmt.Errorf("analysis handler: app: %w", errorsx.ErrRequired)
	}
	return &Handler{app: app}, nil
}

func (h *Handler) NewRun(c *gin.Context) {
	var req newRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errhttp.RespondError(c, fmt.Errorf("%w: %s", errorsx.ErrInvalidArgument, err.Error()))
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		errhttp.RespondError(c, errorsx.ErrInternal)
		return
	}

	input, err := newRunRequestFromRequest(req)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	run, err := h.app.NewRun(c.Request.Context(), userID, input)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, runToResponse(run))
}

func (h *Handler) RunByID(c *gin.Context) {
	runID, err := uuid.Parse(c.Param("run_id"))
	if err != nil {
		errhttp.RespondError(c, fmt.Errorf("run_id: %w", errorsx.ErrInvalidArgument))
		return
	}

	run, err := h.app.RunByID(c.Request.Context(), runID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, runToResponse(run))
}

func (h *Handler) RunArchive(c *gin.Context) {
	runID, err := uuid.Parse(c.Param("run_id"))
	if err != nil {
		errhttp.RespondError(c, fmt.Errorf("run_id: %w", errorsx.ErrInvalidArgument))
		return
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="run_%s.zip"`, runID.String()))

	if err := h.app.StreamRunArchive(c.Request.Context(), runID, c.Writer); err != nil {
		if c.Writer.Written() {
			c.Error(err)
			return
		}
		errhttp.RespondError(c, err)
		return
	}
}

func (h *Handler) ListRuns(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		errhttp.RespondError(c, errorsx.ErrInternal)
		return
	}

	filter, err := appanalysis.ParseRunFilter(c.DefaultQuery("filter", "mine"))
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	limit := int32(20)
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			errhttp.RespondError(c, fmt.Errorf("limit: %w", errorsx.ErrInvalidArgument))
			return
		}
		limit = int32(parsed)
	}

	var beforeID *uuid.UUID
	beforeIDStr := strings.TrimSpace(c.Query("beforeId"))
	if beforeIDStr != "" {
		id, err := uuid.Parse(beforeIDStr)
		if err != nil {
			errhttp.RespondError(c, fmt.Errorf("beforeId: %w", errorsx.ErrInvalidArgument))
			return
		}
		beforeID = &id
	}

	runs, err := h.app.ListRunsPaged(c.Request.Context(), userID, analysis.ListRunsPagedRequest{
		Limit:    limit,
		BeforeID: beforeID,
		Filter:   filter,
	})
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, listRunsResponseToResponse(runs))
}

func (h *Handler) ShareRun(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		errhttp.RespondError(c, errorsx.ErrInternal)
		return
	}

	runID, err := uuid.Parse(c.Param("run_id"))
	if err != nil {
		errhttp.RespondError(c, fmt.Errorf("run_id: %w", errorsx.ErrInvalidArgument))
		return
	}

	if err := h.app.ShareRun(c.Request.Context(), userID, runID); err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"shared": true})
}

func (h *Handler) StreamRun(c *gin.Context) {
	runID, err := uuid.Parse(c.Param("run_id"))
	if err != nil {
		errhttp.RespondError(c, fmt.Errorf("run_id: %w", errorsx.ErrInvalidArgument))
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		errhttp.RespondError(c, fmt.Errorf("%w: streaming not supported", errorsx.ErrInternal))
		return
	}

	ctx := c.Request.Context()
	streamStarted := false

	var respondStreamError func(error)

	sendUpdate := func(run analysis.Run) bool {
		data, _ := json.Marshal(runToResponse(run))
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
		streamStarted = true

		if run.BaseRun.Status.Code == corerun.StatusCodeDone || run.BaseRun.Status.Code == corerun.StatusCodeFailed {
			fmt.Fprintf(c.Writer, "event: done\n\n")
			flusher.Flush()
			return true
		}
		return false
	}

	respondStreamError = func(err error) {
		if !streamStarted && !c.Writer.Written() {
			errhttp.RespondError(c, err)
			return
		}

		c.Error(err)
		_, apiErr := errhttp.MapError(err)
		payload, _ := json.Marshal(apiErr)
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", payload)
		flusher.Flush()
	}

	meta, err := h.app.RunByID(ctx, runID)
	if err != nil {
		respondStreamError(err)
		return
	}
	if sendUpdate(meta) {
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			meta, err := h.app.RunByID(ctx, runID)
			if err != nil {
				respondStreamError(err)
				return
			}

			if sendUpdate(meta) {
				return
			}
		}
	}
}

func (h *Handler) DeleteRun(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		errhttp.RespondError(c, errorsx.ErrInternal)
		return
	}

	runID, err := uuid.Parse(c.Param("run_id"))
	if err != nil {
		errhttp.RespondError(c, fmt.Errorf("run_id: %w", errorsx.ErrInvalidArgument))
		return
	}

	if err := h.app.DeleteRun(c.Request.Context(), userID, runID); err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) ListAvailableExchanges(c *gin.Context) {
	exchanges, err := h.app.ListAvailableExchanges(c.Request.Context())
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, core.ListAvailableExchangesToResponse(exchanges))
}

func (h *Handler) ListAvailableDetectors(c *gin.Context) {
	exchanges, err := h.app.ListAvailableDetectors(c.Request.Context())
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, core.ListAvailableDetectorsToResponse(exchanges))
}

func (h *Handler) Info(c *gin.Context) {
	info, err := h.app.Info(c.Request.Context())
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, core.ServiceInfoToResponse(info))
}

func (h *Handler) Metrics(c *gin.Context) {
	metrics, err := h.app.Metrics(c.Request.Context())
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, core.ServiceMetricsToResponse(metrics))
}
