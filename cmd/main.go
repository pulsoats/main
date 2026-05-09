package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pulsoats/core/tlsconfig"
	"github.com/pulsoats/main/internal/adapters/grpc/analysis"
	grpccatalog "github.com/pulsoats/main/internal/adapters/grpc/catalog"
	grpcsystem "github.com/pulsoats/main/internal/adapters/grpc/system"
	appanalysis "github.com/pulsoats/main/internal/application/analysis"
	appauth "github.com/pulsoats/main/internal/application/auth"
	appmarket "github.com/pulsoats/main/internal/application/market"
	"github.com/pulsoats/main/internal/domain/mailer"
	tokensvc "github.com/pulsoats/main/internal/infrastructure/auth/token"
	"github.com/pulsoats/main/internal/infrastructure/email/aws-ses"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres/auth"
	repomarket "github.com/pulsoats/main/internal/infrastructure/repository/postgres/market"
	"github.com/pulsoats/main/internal/logger"
	analysishandler "github.com/pulsoats/main/internal/transport/handlers/analysis"
	authhandler "github.com/pulsoats/main/internal/transport/handlers/auth"
	markethandler "github.com/pulsoats/main/internal/transport/handlers/market"
	"github.com/pulsoats/main/internal/transport/router"
)

const (
	envHTTPAddr          = "HTTP_ADDR"
	envAppFrontendURL    = "APP_FRONTEND_URL"
	envAppName           = "APP_NAME"
	envJWTSecret         = "JWT_SECRET"
	envRootAdminEmail    = "ROOT_ADMIN_EMAIL"
	envRootAdminPassword = "ROOT_ADMIN_PASSWORD"
	envCORSOrigins       = "CORS_ALLOWED_ORIGINS"
)

const (
	envSESAccessKey    = "SES_ACCESS_KEY"
	envSESSecretKey    = "SES_SECRET_KEY"
	envSESRegion       = "SES_REGION"
	envSESBaseEndpoint = "SES_BASE_ENDPOINT"
	envSESSender       = "SES_SENDER"
)

const (
	analysisGRPCAddr  = "ANALYSIS_GRPC_ADDR"
	envGRPCTLSDisable = "GRPC_TLS_DISABLE"
	envTLSCertFile    = "GRPC_TLS_CERT_FILE"
	envTLSKeyFile     = "GRPC_TLS_KEY_FILE"
	envTLSCAFile      = "GRPC_TLS_CA_FILE"
)

func main() {
	zlogger := logger.Configure()
	slogAdapter := logger.NewSlog(zlogger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPostgresPool(ctx)
	if err != nil {
		zlogger.Fatal().Err(err).Msg("connect postgres")
	}
	defer pool.Close()

	qp := postgres.NewQuerierProvider(pool)
	txManager := postgres.NewTxManager(pool)

	baseURL := strings.TrimSpace(os.Getenv(envAppFrontendURL))
	if baseURL == "" {
		zlogger.Fatal().Msg("APP_FRONTEND_URL is required")
	}
	corsOrigins := parseOrigins(os.Getenv(envCORSOrigins))
	if len(corsOrigins) == 0 {
		corsOrigins = []string{baseURL}
	}

	jwtSecret := strings.TrimSpace(os.Getenv(envJWTSecret))
	if jwtSecret == "" {
		zlogger.Fatal().Msg("JWT_SECRET is required")
	}
	tokenService, err := tokensvc.NewService([]byte(jwtSecret), 15*time.Minute)
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init token service")
	}

	appName := strings.TrimSpace(os.Getenv(envAppName))
	authRepository := auth.NewPostgresRepository(qp)

	emailSender, err := createEmailSender(slogAdapter)
	if err != nil {
		zlogger.Fatal().Msg(err.Error())
	}
	authService, err := appauth.NewService(appauth.ServiceConfig{
		Repository:     authRepository,
		TxManager:      txManager,
		EmailSender:    emailSender,
		TokenService:   tokenService,
		AppFrontendURL: baseURL,
		AppName:        appName,
		Logger:         slogAdapter,
	})
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init auth service")
	}

	rootEmail := strings.TrimSpace(os.Getenv(envRootAdminEmail))
	rootPassword := strings.TrimSpace(os.Getenv(envRootAdminPassword))
	if rootEmail != "" || rootPassword != "" {
		if rootEmail == "" || rootPassword == "" {
			zlogger.Fatal().Msg("both ROOT_ADMIN_EMAIL and ROOT_ADMIN_PASSWORD must be provided")
		}
		if err := authService.EnsureRoot(ctx, rootEmail, rootPassword); err != nil {
			zlogger.Fatal().Err(err).Msg("ensure root admin")
		}
	}

	var grpcTLSCfg *tls.Config
	if strings.TrimSpace(os.Getenv(envGRPCTLSDisable)) != "true" {
		tlsProvider, err := tlsconfig.New(
			os.Getenv(envTLSCertFile),
			os.Getenv(envTLSKeyFile),
			os.Getenv(envTLSCAFile),
		)
		if err != nil {
			zlogger.Fatal().Err(err).Msg("init tls provider")
		}
		grpcTLSCfg = tlsProvider.ClientConfig()
	}

	analysisAddr := strings.TrimSpace(os.Getenv(analysisGRPCAddr))
	if analysisAddr == "" {
		zlogger.Fatal().Msg("ANALYSIS_GRPC_ADDR is required")
	}
	analysisClient, err := analysis.NewClient(analysisAddr, grpcTLSCfg)
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init analysis client")
	}
	analysisCatalogClient, err := grpccatalog.NewClient(analysisAddr, grpcTLSCfg)
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init analysis catalog client")
	}
	analysisSystemClient, err := grpcsystem.NewClient(analysisAddr, grpcTLSCfg)
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init analysis system client")
	}

	marketRepo := repomarket.NewPostgresRepository(qp)
	marketService := appmarket.NewApplication(marketRepo)
	marketHandler := markethandler.NewHandler(marketService)

	analysisService, err := appanalysis.NewApplication(analysisClient, analysisCatalogClient, marketRepo, analysisSystemClient)
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init analysis service")
	}

	authHandler, err := authhandler.NewHandler(authhandler.Config{
		Application: authService,
		BaseURL:     baseURL,
		Logger:      slogAdapter,
	})
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init auth handler")
	}

	analysisHandler, err := analysishandler.NewHandler(analysisService)
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init analysis handler")
	}

	httpRouter, err := router.NewRouter(router.Config{
		AuthHandler:     authHandler,
		MarketHandler:   marketHandler,
		AnalysisHandler: analysisHandler,
		TokenService:    tokenService,
		Logger:          slogAdapter,
		CORSOrigins:     corsOrigins,
	})
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init router")
	}

	addr := strings.TrimSpace(os.Getenv(envHTTPAddr))
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: httpRouter,
	}

	go func() {
		zlogger.Info().Str("addr", addr).Msg("http server started")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			zlogger.Fatal().Err(err).Msg("http server failed")
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		zlogger.Error().Err(err).Msg("shutdown http server")
	}
	zlogger.Info().Msg("server stopped")
}

func createEmailSender(log *slog.Logger) (mailer.Sender, error) {
	accessKey := strings.TrimSpace(os.Getenv(envSESAccessKey))
	if accessKey == "" {
		return nil, fmt.Errorf("SES_ACCESS_KEY is required")
	}

	secretKey := strings.TrimSpace(os.Getenv(envSESSecretKey))
	if secretKey == "" {
		return nil, fmt.Errorf("SES_SECRET_KEY is required")
	}

	region := strings.TrimSpace(os.Getenv(envSESRegion))
	if region == "" {
		return nil, fmt.Errorf("SES_REGION is required")
	}

	baseEndpoint := strings.TrimSpace(os.Getenv(envSESBaseEndpoint))
	if baseEndpoint == "" {
		return nil, fmt.Errorf("SES_BASE_ENDPOINT is required")
	}

	sender := strings.TrimSpace(os.Getenv(envSESSender))
	if sender == "" {
		return nil, fmt.Errorf("SES_SENDER is required")
	}

	cfg := aws_ses.Config{
		BaseEndpoint: baseEndpoint,
		Region:       region,
		AccessKey:    accessKey,
		SecretKey:    secretKey,
		Sender:       sender,
		Logger:       log,
	}

	return aws_ses.NewClient(cfg)
}

func parseOrigins(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		out = append(out, normalizeOrigin(v))
	}
	return out
}

func normalizeOrigin(origin string) string {
	if origin == "*" || strings.HasPrefix(origin, "http://") || strings.HasPrefix(origin, "https://") {
		return origin
	}
	return "https://" + origin
}
