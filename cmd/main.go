package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	coredetectors "github.com/pulsoats/core/domain/detect/detectors"
	"github.com/pulsoats/core/exchanges"
	"github.com/pulsoats/main/internal/adapters/clients/analysisgrpc"
	appanalysis "github.com/pulsoats/main/internal/application/analysis"
	appauth "github.com/pulsoats/main/internal/application/auth"
	appdetectors "github.com/pulsoats/main/internal/application/detectors"
	appmarket "github.com/pulsoats/main/internal/application/market"
	"github.com/pulsoats/main/internal/domain/mailer"
	"github.com/pulsoats/main/internal/infrastructure/email/aws-ses"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres/auth"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres/market"
	"github.com/pulsoats/main/internal/logger"
	analysishandler "github.com/pulsoats/main/internal/transport/handlers/analysis"
	authhandler "github.com/pulsoats/main/internal/transport/handlers/auth"
	detectorshandler "github.com/pulsoats/main/internal/transport/handlers/detectors"
	markethandler "github.com/pulsoats/main/internal/transport/handlers/market"
	"github.com/pulsoats/main/internal/transport/router"
	"github.com/rs/zerolog"
)

const (
	envHTTPAddr          = "HTTP_ADDR"
	envAppBaseURL        = "APP_BASE_URL"
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

const analysisGRPCAddr = "ANALYSIS_GRPC_ADDR"

func main() {
	zlogger := logger.Configure()
	appLog := logger.AsLogx(zlogger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPostgresPool(ctx)
	if err != nil {
		zlogger.Fatal().Err(err).Msg("connect postgres")
	}
	defer pool.Close()

	qp := postgres.NewQuerierProvider(pool)
	txManager := postgres.NewTxManager(pool)

	baseURL := strings.TrimSpace(os.Getenv(envAppBaseURL))
	if baseURL == "" {
		zlogger.Fatal().Msg("APP_BASE_URL is required")
	}
	corsOrigins := parseOrigins(os.Getenv(envCORSOrigins))
	if len(corsOrigins) == 0 {
		corsOrigins = []string{baseURL}
	}

	jwtSecret := strings.TrimSpace(os.Getenv(envJWTSecret))
	if jwtSecret == "" {
		zlogger.Fatal().Msg("JWT_SECRET is required")
	}

	appName := strings.TrimSpace(os.Getenv(envAppName))
	authRepository := auth.NewPostgresRepository(qp)

	emailSender, err := createEmailSender(zlogger)
	if err != nil {
		zlogger.Fatal().Msg(err.Error())
	}
	authService, err := appauth.NewService(appauth.ServiceConfig{
		Repository:  authRepository,
		TxManager:   txManager,
		JWTSecret:   []byte(jwtSecret),
		EmailSender: emailSender,
		AppBaseURL:  baseURL,
		AppName:     appName,
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

	exRegistry := exchanges.NewRegistry(exchanges.WithLogger(appLog))
	marketRepository := market.NewPostgresRepository(qp)
	if err := marketRepository.SyncExchanges(ctx, exRegistry.ListMetadata()); err != nil {
		zlogger.Fatal().Err(err).Msg("sync exchanges in DB")
	}

	marketService, err := appmarket.NewService(appmarket.ServiceConfig{
		Repository:        marketRepository,
		TxManager:         txManager,
		ExchangesRegistry: exRegistry,
	})
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init market service")
	}

	detRegistry := coredetectors.NewDefaultRegistry()
	detectorService, err := appdetectors.NewService(appdetectors.ServiceConfig{
		DetectorsRegistry: detRegistry,
	})
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init detectors service")
	}

	analysisAddr := strings.TrimSpace(os.Getenv(analysisGRPCAddr))
	if analysisAddr == "" {
		zlogger.Fatal().Msg("ANALYSIS_GRPC_ADDR is required")
	}
	analysisClient, err := analysisgrpc.NewClient(analysisAddr)
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init analysis client")
	}
	analysisService := appanalysis.NewService(analysisClient, marketService)

	authHandler, err := authhandler.NewHandler(authhandler.Config{
		Service: authService,
		BaseURL: baseURL,
		Logger:  appLog,
	})
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init auth handler")
	}

	marketHandler := markethandler.NewHandler(marketService)
	detHandler := detectorshandler.NewHandler(detectorService)
	analysisHandler := analysishandler.NewHandler(analysisService)

	httpRouter, err := router.NewRouter(router.Config{
		AuthHandler:      authHandler,
		DetectorsHandler: detHandler,
		MarketHandler:    marketHandler,
		AnalysisHandler:  analysisHandler,
		JWTSecret:        []byte(jwtSecret),
		Logger:           appLog,
		CORSOrigins:      corsOrigins,
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

func createEmailSender(log zerolog.Logger) (mailer.Sender, error) {
	accessKey := strings.TrimSpace(os.Getenv(envSESAccessKey))
	if accessKey == "" {
		log.Fatal().Msg("SES_ACCESS_KEY is required")
	}

	secretKey := strings.TrimSpace(os.Getenv(envSESSecretKey))
	if secretKey == "" {
		log.Fatal().Msg("SES_SECRET_KEY is required")
	}

	region := strings.TrimSpace(os.Getenv(envSESRegion))
	if region == "" {
		log.Fatal().Msg("SES_REGION is required")
	}

	baseEndpoint := strings.TrimSpace(os.Getenv(envSESBaseEndpoint))
	if baseEndpoint == "" {
		log.Fatal().Msg("SES_BASE_ENDPOINT is required")
	}

	sender := strings.TrimSpace(os.Getenv(envSESSender))
	if sender == "" {
		log.Fatal().Msg("SES_SENDER is required")
	}

	cfg := aws_ses.Config{
		BaseEndpoint: baseEndpoint,
		Region:       region,
		AccessKey:    accessKey,
		SecretKey:    secretKey,
		Sender:       sender,
		Logger:       logger.AsLogx(log),
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
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
