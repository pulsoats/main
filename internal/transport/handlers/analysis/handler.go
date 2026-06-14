package analysis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
	corerun "github.com/pulsoats/core/run"
	appanalysis "github.com/pulsoats/main/internal/application/analysis"
	"github.com/pulsoats/main/internal/domain/analysis"
	"github.com/pulsoats/main/internal/transport/errhttp"
	"github.com/pulsoats/main/internal/transport/handlers/core"
	"github.com/pulsoats/main/internal/transport/middleware"
)

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

func (h *Handler) RunsPaged(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		errhttp.RespondError(c, errorsx.ErrInternal)
		return
	}

	var req runsPagedRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		errhttp.RespondError(c, errorsx.ErrInvalidArgument)
		return
	}

	reqFromDTO, err := runsPagedRequestFromDTO(req)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	runs, err := h.app.RunsPaged(c.Request.Context(), userID, reqFromDTO)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, runsPagedResponseToResponse(runs))
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

func (h *Handler) AvailableExchanges(c *gin.Context) {
	exchanges, err := h.app.AvailableExchanges(c.Request.Context())
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, core.AvailableExchangesToResponse(exchanges))
}

func (h *Handler) AvailableDetectors(c *gin.Context) {
	exchanges, err := h.app.AvailableDetectors(c.Request.Context())
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, core.AvailableDetectorsToResponse(exchanges))
}
