package router

import (
	"fmt"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/ports"
	"github.com/pulsoats/main/internal/transport/handlers/analysis"
	"github.com/pulsoats/main/internal/transport/handlers/auth"
	"github.com/pulsoats/main/internal/transport/handlers/detectors"
	"github.com/pulsoats/main/internal/transport/handlers/market"
	"github.com/pulsoats/main/internal/transport/middleware"
)

type Config struct {
	AuthHandler      *auth.Handler
	DetectorsHandler *detectors.Handler
	MarketHandler    *market.Handler
	AnalysisHandler  *analysis.Handler
	TokenService     ports.TokenService
	Logger           *slog.Logger
	CORSOrigins      []string
}

func NewRouter(cfg Config) (*gin.Engine, error) {
	if cfg.AuthHandler == nil {
		return nil, fmt.Errorf("new router: auth handler: %w", errorsx.ErrRequired)
	}
	if cfg.DetectorsHandler == nil {
		return nil, fmt.Errorf("new router: detectors handler: %w", errorsx.ErrInvalidArgument)
	}
	if cfg.MarketHandler == nil {
		return nil, fmt.Errorf("new router: market handler: %w", errorsx.ErrInvalidArgument)
	}
	if cfg.Logger == nil {
		return nil, fmt.Errorf("new router: %w: logger", errorsx.ErrInvalidArgument)
	}
	if cfg.TokenService == nil {
		return nil, fmt.Errorf("new router: token service: %w", errorsx.ErrRequired)
	}
	if len(cfg.CORSOrigins) == 0 {
		return nil, fmt.Errorf("new router: %w: cors origins", errorsx.ErrInvalidArgument)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.LoggerMiddleware(cfg.Logger))

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
	marketGroup.GET("/exchanges/meta", cfg.MarketHandler.ListExchangeMetas)
	marketGroup.GET("/symbols", cfg.MarketHandler.ListSymbols)

	detectorsGroup := r.Group("/detectors")
	detectorsGroup.Use(middleware.AuthMiddleware(cfg.TokenService))
	detectorsGroup.GET("/meta", cfg.DetectorsHandler.ListMetas)

	analysisGroup := r.Group("/runs")
	analysisGroup.Use(middleware.AuthMiddleware(cfg.TokenService))
	analysisGroup.GET("", cfg.AnalysisHandler.ListRuns)
	analysisGroup.POST("", cfg.AnalysisHandler.StartRun)
	analysisGroup.GET("/:run_id/meta", cfg.AnalysisHandler.RunMeta)
	analysisGroup.GET("/:run_id/meta/stream", cfg.AnalysisHandler.StreamRunMeta)
	analysisGroup.GET("/:run_id/result", cfg.AnalysisHandler.RunResult)
	analysisGroup.PATCH("/:run_id/share", cfg.AnalysisHandler.ShareRun)

	return r, nil
}
