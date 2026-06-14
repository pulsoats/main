package router

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/pulsoats/main/internal/ports"
	"github.com/pulsoats/main/internal/transport/handlers/analysis"
	"github.com/pulsoats/main/internal/transport/handlers/auth"
	livehandler "github.com/pulsoats/main/internal/transport/handlers/live"
	"github.com/pulsoats/main/internal/transport/handlers/market"
	"github.com/pulsoats/main/internal/transport/middleware"
)

type Config struct {
	AuthHandler     *auth.Handler
	MarketHandler   *market.Handler
	AnalysisHandler *analysis.Handler
	LiveHandler     *livehandler.Handler
	TokenService    ports.TokenService
	Logger          *slog.Logger
	CORSOrigins     []string
}

func NewRouter(cfg Config) (*gin.Engine, error) {
	if cfg.AuthHandler == nil {
		return nil, fmt.Errorf("router: auth handler is nil")
	}
	if cfg.MarketHandler == nil {
		return nil, fmt.Errorf("router: market handler is nil")
	}
	if cfg.AnalysisHandler == nil {
		return nil, fmt.Errorf("router: analysis handler is nil")
	}
	if cfg.LiveHandler == nil {
		return nil, fmt.Errorf("router: live handler is nil")
	}
	if cfg.TokenService == nil {
		return nil, fmt.Errorf("router: token service is nil")
	}
	if len(cfg.CORSOrigins) == 0 {
		return nil, fmt.Errorf("router: cors origins is empty")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware(cfg.CORSOrigins))
	r.Use(middleware.LoggerMiddleware(logger))

	// Public auth routes
	authGroup := r.Group("/auth")
	authGroup.POST("/register", cfg.AuthHandler.Register)
	authGroup.POST("/login", cfg.AuthHandler.Login)
	authGroup.GET("/verify", cfg.AuthHandler.VerifyEmail)
	authGroup.POST("/refresh", cfg.AuthHandler.RefreshToken)
	authGroup.POST("/password/reset/request", cfg.AuthHandler.RequestPasswordReset)
	authGroup.POST("/password/reset", cfg.AuthHandler.ResetPassword)

	// все остальное под AuthMiddleware
	api := r.Group("")
	api.Use(middleware.AuthMiddleware(cfg.TokenService))

	// Auth protected
	api.GET("/auth/sessions", cfg.AuthHandler.ActiveSessions)
	api.DELETE("/auth/sessions/:session_id", cfg.AuthHandler.LogoutBySessionID)
	api.GET("/auth/profile", cfg.AuthHandler.Profile)
	api.DELETE("/auth/logout", cfg.AuthHandler.Logout)
	api.DELETE("/auth/logout-all", cfg.AuthHandler.LogoutAll)
	api.POST("/auth/password/change", cfg.AuthHandler.ChangePassword)

	// Admin
	admin := api.Group("/admin")
	admin.Use(middleware.AdminOnlyMiddleware())
	admin.POST("/invite-tokens", cfg.AuthHandler.CreateInviteToken)
	admin.GET("/invite-tokens", cfg.AuthHandler.InviteTokens)
	admin.DELETE("/invite-tokens/:token_id", cfg.AuthHandler.RevokeInviteToken)
	admin.POST("/nodes", cfg.LiveHandler.CreateNode)

	// Market
	marketGroup := api.Group("/market")
	marketGroup.GET("/symbols", cfg.MarketHandler.Symbols)
	marketGroup.GET("/symbols/suggest", cfg.MarketHandler.Suggest)

	// Analysis
	analysisGroup := api.Group("/analysis")
	analysisGroup.GET("/catalog/exchanges", cfg.AnalysisHandler.AvailableExchanges)
	analysisGroup.GET("/catalog/detectors", cfg.AnalysisHandler.AvailableDetectors)
	analysisRuns := analysisGroup.Group("/runs")
	analysisRuns.GET("", cfg.AnalysisHandler.RunsPaged)
	analysisRuns.POST("", cfg.AnalysisHandler.NewRun)
	analysisRuns.GET("/:run_id", cfg.AnalysisHandler.RunByID)
	analysisRuns.GET("/:run_id/stream", cfg.AnalysisHandler.StreamRun)
	analysisRuns.GET("/:run_id/result", cfg.AnalysisHandler.RunArchive)
	analysisRuns.PATCH("/:run_id/share", cfg.AnalysisHandler.ShareRun)
	analysisRuns.DELETE("/:run_id", cfg.AnalysisHandler.DeleteRun)

	// Nodes (admin)
	admin.GET("/nodes", cfg.LiveHandler.Nodes)
	admin.GET("/nodes/:node_id", cfg.LiveHandler.NodeByID)
	admin.POST("/nodes/:node_id/disable", cfg.LiveHandler.DisableNode)
	admin.POST("/nodes/:node_id/enable", cfg.LiveHandler.EnableNode)
	admin.DELETE("/nodes/:node_id", cfg.LiveHandler.DeleteNode)

	// Workers (global list)
	api.GET("/workers", cfg.LiveHandler.Workers)

	// Accounts
	api.GET("/accounts", cfg.LiveHandler.Accounts)
	api.POST("/accounts", cfg.LiveHandler.CreateAccount)

	acc := api.Group("/accounts/:account_id")
	acc.GET("", cfg.LiveHandler.AccountByID)
	acc.PATCH("/name", cfg.LiveHandler.UpdateAccountName)
	acc.GET("/catalog/exchanges", cfg.LiveHandler.AvailableExchanges)
	acc.GET("/catalog/detectors", cfg.LiveHandler.AvailableDetectors)
	acc.GET("/signals", cfg.LiveHandler.SignalsPaged)
	acc.GET("/events", cfg.LiveHandler.StreamEvents)
	acc.GET("/worker", cfg.LiveHandler.WorkerByAccountID)
	acc.POST("/worker", cfg.LiveHandler.CreateWorker)
	acc.POST("/worker/start", cfg.LiveHandler.StartWorker)
	acc.POST("/worker/stop", cfg.LiveHandler.StopWorker)
	acc.GET("/worker/metrics", cfg.LiveHandler.StreamWorkerMetrics)
	acc.GET("/worker/stats", cfg.LiveHandler.StreamWorkerStats)

	accRuns := acc.Group("/runs")
	accRuns.GET("", cfg.LiveHandler.RunsPaged)
	accRuns.POST("", cfg.LiveHandler.NewRun)
	accRuns.GET("/:run_id", cfg.LiveHandler.GetRun)
	accRuns.POST("/:run_id/restart", cfg.LiveHandler.RestartRun)
	accRuns.DELETE("/:run_id", cfg.LiveHandler.StopRun)
	accRuns.DELETE("", cfg.LiveHandler.StopAll)

	return r, nil
}
