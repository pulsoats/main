package router

import (
	"fmt"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/lib/logx"
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
	JWTSecret        []byte
	Logger           logx.Logger
	CORSOrigins      []string
}

func NewRouter(cfg Config) (*gin.Engine, error) {
	if cfg.AuthHandler == nil {
		return nil, fmt.Errorf("new router: %w: auth handler", derrors.ErrRequired)
	}
	if cfg.DetectorsHandler == nil {
		return nil, fmt.Errorf("new router: %w: detectors handler", derrors.ErrRequired)
	}
	if cfg.MarketHandler == nil {
		return nil, fmt.Errorf("new router: %w: market handler", derrors.ErrRequired)
	}
	if cfg.JWTSecret == nil || len(cfg.JWTSecret) == 0 {
		return nil, fmt.Errorf("new router: %w: jwt secret", derrors.ErrRequired)
	}
	if cfg.Logger == nil {
		return nil, fmt.Errorf("new router: %w: logger", derrors.ErrRequired)
	}
	if len(cfg.CORSOrigins) == 0 {
		return nil, fmt.Errorf("new router: %w: cors origins", derrors.ErrRequired)
	}

	r := gin.New()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	r.Use(gin.Recovery())
	r.Use(middleware.LoggerMiddleware(cfg.Logger))

	public := r.Group("/auth")
	public.POST("/register", cfg.AuthHandler.Register)
	public.POST("/login", cfg.AuthHandler.Login)
	public.GET("/verify", cfg.AuthHandler.VerifyEmail)

	protected := public.Group("")
	protected.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	protected.POST("/refresh", cfg.AuthHandler.RefreshToken)
	protected.POST("/password/change", cfg.AuthHandler.ChangePassword)
	protected.POST("/password/reset/request", cfg.AuthHandler.RequestPasswordReset)
	protected.POST("/password/reset", cfg.AuthHandler.ResetPassword)

	admin := protected.Group("/admin")
	admin.Use(middleware.AdminOnlyMiddleware())
	admin.POST("/invite-token", cfg.AuthHandler.CreateInviteToken)

	marketGroup := r.Group("/market")
	marketGroup.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	marketGroup.GET("/exchanges", cfg.MarketHandler.ListExchangeMetas)
	marketGroup.GET("/symbols", cfg.MarketHandler.ListSymbols)

	detectorsGroup := r.Group("/detectors")
	detectorsGroup.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	detectorsGroup.GET("", cfg.DetectorsHandler.ListMetas)

	analysisGroup := r.Group("/runs")
	analysisGroup.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	analysisGroup.POST("", cfg.AnalysisHandler.StartRun)
	analysisGroup.GET("/:run_id/status", cfg.AnalysisHandler.RunStatus)
	analysisGroup.GET("/:run_id/meta", cfg.AnalysisHandler.RunMeta)
	analysisGroup.GET("/:run_id/result", cfg.AnalysisHandler.RunResult)

	return r, nil
}
