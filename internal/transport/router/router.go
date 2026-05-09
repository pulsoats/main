package router

import (
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/pulsoats/main/internal/ports"
	"github.com/pulsoats/main/internal/transport/handlers/analysis"
	"github.com/pulsoats/main/internal/transport/handlers/auth"
	"github.com/pulsoats/main/internal/transport/handlers/market"
	"github.com/pulsoats/main/internal/transport/middleware"
)

type Config struct {
	AuthHandler     *auth.Handler
	MarketHandler   *market.Handler
	AnalysisHandler *analysis.Handler
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

	public := r.Group("/auth")
	public.POST("/register", cfg.AuthHandler.Register)
	public.POST("/login", cfg.AuthHandler.Login)
	public.GET("/verify", cfg.AuthHandler.VerifyEmail)
	public.POST("/refresh", cfg.AuthHandler.RefreshToken)
	public.POST("/password/reset/request", cfg.AuthHandler.RequestPasswordReset)
	public.POST("/password/reset", cfg.AuthHandler.ResetPassword)

	protected := public.Group("")
	protected.Use(middleware.AuthMiddleware(cfg.TokenService))
	protected.GET("/sessions", cfg.AuthHandler.ListActiveSessions)
	protected.DELETE("/sessions/:session_id", cfg.AuthHandler.LogoutBySessionID)
	protected.GET("/profile", cfg.AuthHandler.Profile)
	protected.DELETE("/logout", cfg.AuthHandler.Logout)
	protected.DELETE("/logout-all", cfg.AuthHandler.LogoutAll)
	protected.POST("/password/change", cfg.AuthHandler.ChangePassword)

	admin := r.Group("/admin")
	admin.Use(middleware.AuthMiddleware(cfg.TokenService), middleware.AdminOnlyMiddleware())
	admin.POST("/invite-tokens", cfg.AuthHandler.CreateInviteToken)
	admin.GET("/invite-tokens", cfg.AuthHandler.ListInviteTokens)
	admin.DELETE("/invite-tokens/:token_id", cfg.AuthHandler.RevokeInviteToken)

	marketGroup := r.Group("/market")
	marketGroup.Use(middleware.AuthMiddleware(cfg.TokenService))
	marketGroup.GET("/exchanges/meta", cfg.AnalysisHandler.ListAvailableExchanges)
	marketGroup.GET("/symbols", cfg.MarketHandler.ListSymbols)
	marketGroup.GET("/symbols/suggest", cfg.MarketHandler.Suggest)

	detGroup := r.Group("/detectors")
	detGroup.Use(middleware.AuthMiddleware(cfg.TokenService))
	detGroup.GET("/meta", cfg.AnalysisHandler.ListAvailableDetectors)

	analysisGroup := r.Group("/analysis")
	analysisGroup.Use(middleware.AuthMiddleware(cfg.TokenService))

	analysisRunsGroup := analysisGroup.Group("/runs")
	analysisRunsGroup.GET("", cfg.AnalysisHandler.ListRuns)
	analysisRunsGroup.POST("", cfg.AnalysisHandler.NewRun)
	analysisRunsGroup.GET("/:run_id", cfg.AnalysisHandler.RunByID)
	analysisRunsGroup.GET("/:run_id/stream", cfg.AnalysisHandler.StreamRun)
	analysisRunsGroup.GET("/:run_id/result", cfg.AnalysisHandler.RunArchive)
	analysisRunsGroup.PATCH("/:run_id/share", cfg.AnalysisHandler.ShareRun)
	analysisRunsGroup.DELETE("/:run_id", cfg.AnalysisHandler.DeleteRun)

	analysisGroup.GET("/info", cfg.AnalysisHandler.Info)
	analysisGroup.GET("/metrics", cfg.AnalysisHandler.Metrics)

	return r, nil
}
