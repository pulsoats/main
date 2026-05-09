package live

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pulsoats/core/detect"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/exchange"
	coremarket "github.com/pulsoats/core/market"
	coresystem "github.com/pulsoats/core/system"
	domainlive "github.com/pulsoats/main/internal/domain/live"
	domainsystem "github.com/pulsoats/main/internal/domain/system"
	"github.com/pulsoats/main/internal/transport/errhttp"
	"github.com/pulsoats/main/internal/transport/handlers/core"
	"github.com/pulsoats/main/internal/transport/middleware"
)

type app interface {
	Register(ctx context.Context, addr string) (coresystem.ServiceInfo, error)
	Remove(ctx context.Context, exchange, account string) error
	ListServices(ctx context.Context) ([]domainsystem.Service, error)
	ServiceIDByExchangeAccount(exchange, account string) (uuid.UUID, bool)

	NewRun(ctx context.Context, serviceID uuid.UUID, mkt coremarket.Spec, interval string, detector detect.DetectorConfig, callerID uuid.UUID) (domainlive.Run, error)
	RestartRun(ctx context.Context, serviceID uuid.UUID, runID uuid.UUID, callerID uuid.UUID) (domainlive.Run, error)
	StopRun(ctx context.Context, serviceID uuid.UUID, runID uuid.UUID, callerID uuid.UUID) error
	StopAll(ctx context.Context, serviceID uuid.UUID, callerID uuid.UUID) error
	GetRun(ctx context.Context, serviceID uuid.UUID, runID uuid.UUID) (domainlive.Run, error)
	ListRunsPaged(ctx context.Context, serviceID uuid.UUID, req domainlive.ListRunsRequest) (domainlive.ListRunsResponse, error)
	ListSignalsPaged(ctx context.Context, serviceID uuid.UUID, req domainlive.ListSignalsPagedRequest) (domainlive.ListSignalsPagedResponse, error)
	StreamEvents(ctx context.Context, serviceID uuid.UUID) (*domainlive.Stream, error)

	ListAvailableDetectors(ctx context.Context, serviceID uuid.UUID) ([]detect.DetectorMeta, error)
	ListAvailableExchanges(ctx context.Context, serviceID uuid.UUID) ([]exchange.Meta, error)

	Info(ctx context.Context, serviceID uuid.UUID) (coresystem.ServiceInfo, error)
	Metrics(ctx context.Context, serviceID uuid.UUID) (coresystem.ServiceMetrics, error)
}

type Handler struct {
	app app
}

func NewHandler(app app) (*Handler, error) {
	if app == nil {
		return nil, fmt.Errorf("live handler: app: %w", errorsx.ErrRequired)
	}
	return &Handler{app: app}, nil
}

func (h *Handler) resolveServiceID(c *gin.Context) (uuid.UUID, bool) {
	exchange := c.Param("exchange")
	account := c.Param("account")
	id, ok := h.app.ServiceIDByExchangeAccount(exchange, account)
	if !ok {
		errhttp.RespondError(c, fmt.Errorf("service %s/%s: %w", exchange, account, errorsx.ErrNotFound))
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) RegisterService(c *gin.Context) {
	var req registerServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errhttp.RespondError(c, fmt.Errorf("%w: %s", errorsx.ErrInvalidArgument, err.Error()))
		return
	}

	info, err := h.app.Register(c.Request.Context(), req.Addr)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, core.ServiceInfoToResponse(info))
}

func (h *Handler) RemoveService(c *gin.Context) {
	exchange := c.Param("exchange")
	account := c.Param("account")

	if err := h.app.Remove(c.Request.Context(), exchange, account); err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) ListServices(c *gin.Context) {
	services, err := h.app.ListServices(c.Request.Context())
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, servicesToResponse(services))
}

func (h *Handler) ServiceInfo(c *gin.Context) {
	serviceID, ok := h.resolveServiceID(c)
	if !ok {
		return
	}

	info, err := h.app.Info(c.Request.Context(), serviceID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, core.ServiceInfoToResponse(info))
}

func (h *Handler) ServiceMetrics(c *gin.Context) {
	serviceID, ok := h.resolveServiceID(c)
	if !ok {
		return
	}

	metrics, err := h.app.Metrics(c.Request.Context(), serviceID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, core.ServiceMetricsToResponse(metrics))
}

// --- catalog ---

func (h *Handler) ListAvailableExchanges(c *gin.Context) {
	serviceID, ok := h.resolveServiceID(c)
	if !ok {
		return
	}

	exchanges, err := h.app.ListAvailableExchanges(c.Request.Context(), serviceID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, core.ListAvailableExchangesToResponse(exchanges))
}

func (h *Handler) ListAvailableDetectors(c *gin.Context) {
	serviceID, ok := h.resolveServiceID(c)
	if !ok {
		return
	}

	detectors, err := h.app.ListAvailableDetectors(c.Request.Context(), serviceID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, core.ListAvailableDetectorsToResponse(detectors))
}

// --- runs ---

func (h *Handler) NewRun(c *gin.Context) {
	serviceID, ok := h.resolveServiceID(c)
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

	run, err := h.app.NewRun(c.Request.Context(), serviceID,
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
	serviceID, ok := h.resolveServiceID(c)
	if !ok {
		return
	}

	runID, err := uuid.Parse(c.Param("run_id"))
	if err != nil {
		errhttp.RespondError(c, fmt.Errorf("run_id: %w", errorsx.ErrInvalidArgument))
		return
	}

	run, err := h.app.GetRun(c.Request.Context(), serviceID, runID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, runToResponse(run))
}

func (h *Handler) RestartRun(c *gin.Context) {
	serviceID, ok := h.resolveServiceID(c)
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

	run, err := h.app.RestartRun(c.Request.Context(), serviceID, runID, callerID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, runToResponse(run))
}

func (h *Handler) StopRun(c *gin.Context) {
	serviceID, ok := h.resolveServiceID(c)
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

	if err := h.app.StopRun(c.Request.Context(), serviceID, runID, callerID); err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) StopAll(c *gin.Context) {
	serviceID, ok := h.resolveServiceID(c)
	if !ok {
		return
	}

	callerID, ok := middleware.GetUserID(c)
	if !ok {
		errhttp.RespondError(c, errorsx.ErrInternal)
		return
	}

	if err := h.app.StopAll(c.Request.Context(), serviceID, callerID); err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) ListRuns(c *gin.Context) {
	serviceID, ok := h.resolveServiceID(c)
	if !ok {
		return
	}

	var q listRunsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		errhttp.RespondError(c, fmt.Errorf("%w: %s", errorsx.ErrInvalidArgument, err.Error()))
		return
	}
	if q.Limit <= 0 {
		q.Limit = 20
	}

	resp, err := h.app.ListRunsPaged(c.Request.Context(), serviceID, domainlive.ListRunsRequest{
		Limit:    q.Limit,
		BeforeID: q.BeforeID,
	})
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, listRunsToResponse(resp))
}

func (h *Handler) ListSignals(c *gin.Context) {
	serviceID, ok := h.resolveServiceID(c)
	if !ok {
		return
	}

	var q listSignalsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		errhttp.RespondError(c, fmt.Errorf("%w: %s", errorsx.ErrInvalidArgument, err.Error()))
		return
	}
	if q.Limit <= 0 {
		q.Limit = 50
	}

	filter := &domainlive.ListSignalsFilter{RunID: q.RunID}

	resp, err := h.app.ListSignalsPaged(c.Request.Context(), serviceID, domainlive.ListSignalsPagedRequest{
		Limit:    q.Limit,
		BeforeID: q.BeforeID,
		Filter:   filter,
	})
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, listSignalsToResponse(resp))
}

// --- SSE stream ---

func (h *Handler) StreamEvents(c *gin.Context) {
	serviceID, ok := h.resolveServiceID(c)
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

	stream, err := h.app.StreamEvents(c.Request.Context(), serviceID)
	if err != nil {
		errhttp.RespondError(c, err)
		return
	}

	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, open := <-stream.Events:
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
