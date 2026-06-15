package live

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/application/live"
	domainlive "github.com/pulsoats/main/internal/domain/live"
	"github.com/pulsoats/main/internal/transport/errhttp"
	"github.com/pulsoats/main/internal/transport/handlers/core"
	"github.com/pulsoats/main/internal/transport/middleware"
)

func (h *Handler) CreateWorker(c *gin.Context) {
	exchangeAccountID, err := uuid.Parse(c.Param("account_id"))
	if err != nil {
		errhttp.RespondError(c, errorsx.ErrInvalidArgument)
		return
	}

	worker, err := h.app.CreateWorker(c.Request.Context(), exchangeAccountID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, workerToResponse(worker))
}

func (h *Handler) StartWorker(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	worker, err := h.app.StartWorker(c.Request.Context(), accountID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, workerToResponse(worker))
}

func (h *Handler) UpdateWorker(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	worker, err := h.app.UpdateWorker(c.Request.Context(), accountID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, workerToResponse(worker))
}

func (h *Handler) StopWorker(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	callerID, ok := middleware.GetUserID(c)
	if !ok {
		errhttp.RespondError(c, errorsx.ErrForbidden)
		return
	}

	if err := h.app.StopWorker(c.Request.Context(), accountID, callerID); err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) WorkerByAccountID(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	worker, err := h.app.WorkerByExchangeAccountID(c.Request.Context(), accountID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, workerToResponse(worker))
}

func (h *Handler) Workers(c *gin.Context) {
	exchange := c.Query("exchange")
	nodeIDStr := c.Query("node_id")

	if exchange != "" && nodeIDStr != "" {
		errhttp.RespondError(c, fmt.Errorf("exchange and node_id are mutually exclusive: %w", errorsx.ErrInvalidArgument))
		return
	}

	var f live.WorkersFilter
	if exchange != "" {
		f.Exchange = &exchange
	}
	if nodeIDStr != "" {
		nodeID, err := uuid.Parse(nodeIDStr)
		if err != nil {
			errhttp.RespondError(c, fmt.Errorf("node_id: %w", errorsx.ErrInvalidArgument))
			return
		}
		f.NodeID = &nodeID
	}

	workers, err := h.app.Workers(c.Request.Context(), f)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, workersToResponse(workers))
}

func (h *Handler) StreamWorkerMetrics(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
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

	ch, err := h.app.SubscribeWorkerMetrics(ctx, accountID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case metrics, open := <-ch:
			if !open {
				fmt.Fprintf(c.Writer, "event: done\ndata: {}\n\n")
				flusher.Flush()
				return
			}
			data, err := json.Marshal(metrics)
			if err != nil {
				continue
			}
			fmt.Fprintf(c.Writer, "event: metrics\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (h *Handler) AvailableExchanges(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	exchanges, err := h.app.AvailableExchanges(c.Request.Context(), accountID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, core.AvailableExchangesToResponse(exchanges))
}

func (h *Handler) AvailableDetectors(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	detectors, err := h.app.AvailableDetectors(c.Request.Context(), accountID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, core.AvailableDetectorsToResponse(detectors))
}

func (h *Handler) NewRun(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	callerID, ok := middleware.GetUserID(c)
	if !ok {
		errhttp.RespondError(c, errorsx.ErrInternal)
		return
	}

	var req newRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errhttp.RespondError(c, fmt.Errorf("%w: %s", errorsx.ErrInvalidArgument, err.Error()))
		return
	}

	run, err := h.app.NewRun(c.Request.Context(), accountID,
		core.MarketSpecFromRequest(req.Market),
		req.Interval,
		core.DetectorConfigFromRequest(req.Detector),
		callerID,
	)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, runToResponse(run))
}

func (h *Handler) GetRun(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	runID, err := uuid.Parse(c.Param("run_id"))
	if err != nil {
		errhttp.RespondError(c, fmt.Errorf("run_id: %w", errorsx.ErrInvalidArgument))
		return
	}

	run, err := h.app.Run(c.Request.Context(), accountID, runID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, runToResponse(run))
}

func (h *Handler) RestartRun(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	callerID, ok := middleware.GetUserID(c)
	if !ok {
		errhttp.RespondError(c, errorsx.ErrInternal)
		return
	}

	runID, err := uuid.Parse(c.Param("run_id"))
	if err != nil {
		errhttp.RespondError(c, fmt.Errorf("run_id: %w", errorsx.ErrInvalidArgument))
		return
	}

	run, err := h.app.RestartRun(c.Request.Context(), accountID, runID, callerID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, runToResponse(run))
}

func (h *Handler) StopRun(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	callerID, ok := middleware.GetUserID(c)
	if !ok {
		errhttp.RespondError(c, errorsx.ErrInternal)
		return
	}

	runID, err := uuid.Parse(c.Param("run_id"))
	if err != nil {
		errhttp.RespondError(c, fmt.Errorf("run_id: %w", errorsx.ErrInvalidArgument))
		return
	}

	if err := h.app.StopRun(c.Request.Context(), accountID, runID, callerID); err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) StopAll(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	callerID, ok := middleware.GetUserID(c)
	if !ok {
		errhttp.RespondError(c, errorsx.ErrInternal)
		return
	}

	if err := h.app.StopAllRuns(c.Request.Context(), accountID, callerID); err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) RunsPaged(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	var req runsPagedRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		errhttp.RespondError(c, fmt.Errorf("%w: %s", errorsx.ErrInvalidArgument, err.Error()))
		return
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}

	domainReq, err := runsPagedRequestFromDTO(req)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	resp, err := h.app.RunsPaged(c.Request.Context(), accountID, domainReq)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, runsPagedToResponse(resp))
}

func (h *Handler) SignalsPaged(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
		return
	}

	var req signalsPagedRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		errhttp.RespondError(c, fmt.Errorf("%w: %s", errorsx.ErrInvalidArgument, err.Error()))
		return
	}

	domainReq, err := signalsPagedRequestFromDTO(req)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	resp, err := h.app.SignalsPaged(c.Request.Context(), accountID, domainReq)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, signalsPagedToResponse(resp))
}

// --- SSE stream ---

func (h *Handler) StreamEvents(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
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

	events, err := h.app.StreamEvents(c.Request.Context(), accountID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, open := <-events:
			if !open {
				fmt.Fprintf(c.Writer, "event: done\ndata: {}\n\n")
				flusher.Flush()
				return
			}

			var eventType string
			var payload any

			switch p := event.Payload.(type) {
			case domainlive.RunEvent:
				eventType = "run"
				payload = runToResponse(p.Run)
			case domainlive.SignalEvent:
				eventType = "signal"
				payload = signalToResponse(p.Signal)
			default:
				continue
			}

			data, err := json.Marshal(payload)
			if err != nil {
				continue
			}
			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", eventType, data)
			flusher.Flush()
		}
	}
}

func (h *Handler) StreamWorkerStats(c *gin.Context) {
	accountID, ok := h.resolveAccountID(c)
	if !ok {
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

	statsCh, err := h.app.SubscribeWorkerStats(c.Request.Context(), accountID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case stats, open := <-statsCh:
			if !open {
				fmt.Fprintf(c.Writer, "event: done\ndata: {}\n\n")
				flusher.Flush()
				return
			}
			data, err := json.Marshal(workerStatsToResponse(stats))
			if err != nil {
				continue
			}
			fmt.Fprintf(c.Writer, "event: stats\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}
