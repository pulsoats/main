package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/pulsoats/core/tlsconfig"
	appanalysis "github.com/pulsoats/main/internal/application/analysis"
	appauth "github.com/pulsoats/main/internal/application/auth"
	applive "github.com/pulsoats/main/internal/application/live"
	appmarket "github.com/pulsoats/main/internal/application/market"
	"github.com/pulsoats/main/internal/domain/mailer"
	tokensvc "github.com/pulsoats/main/internal/infrastructure/auth/token"
	"github.com/pulsoats/main/internal/infrastructure/certgen"
	"github.com/pulsoats/main/internal/infrastructure/cryptox"
	"github.com/pulsoats/main/internal/infrastructure/docker"
	awsses "github.com/pulsoats/main/internal/infrastructure/email/aws-ses"
	"github.com/pulsoats/main/internal/infrastructure/grpc/analysis"
	grpclive "github.com/pulsoats/main/internal/infrastructure/grpc/live"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres/auth"
	repolive "github.com/pulsoats/main/internal/infrastructure/repository/postgres/live"
	repomarket "github.com/pulsoats/main/internal/infrastructure/repository/postgres/market"
	"github.com/pulsoats/main/internal/logger"
	analysishandler "github.com/pulsoats/main/internal/transport/handlers/analysis"
	authhandler "github.com/pulsoats/main/internal/transport/handlers/auth"
	livehandler "github.com/pulsoats/main/internal/transport/handlers/live"
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
	envAnalysisGRPCAddr = "ANALYSIS_GRPC_ADDR"
	envGRPCTLSDisable   = "GRPC_TLS_DISABLE"
	envGRPCCertFile     = "GRPC_CERT_FILE"
	envGRPCKeyFile      = "GRPC_KEY_FILE"
	envGRPCCAFile       = "GRPC_CA_FILE"
	envGRPCCAKeyFile    = "GRPC_CA_KEY_FILE"
)

const (
	envGHCRUser      = "GHCR_USER"
	envGHCRToken     = "GHCR_TOKEN"
	envLiveImageURL  = "LIVE_IMAGE_URL"
	envDockerDBImage = "DOCKER_DB_IMAGE"
	envDockerCACert  = "DOCKER_CA_CERT"
	envDockerCert    = "DOCKER_CERT"
	envDockerKey     = "DOCKER_KEY"
)

const (
	envCredentialsKey = "CREDENTIALS_KEY" // hex-encoded 32-byte AES key
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
			os.Getenv(envGRPCCertFile),
			os.Getenv(envGRPCKeyFile),
			os.Getenv(envGRPCCAFile),
		)
		if err != nil {
			zlogger.Fatal().Err(err).Msg("init tls provider")
		}
		grpcTLSCfg = tlsProvider.ClientConfig()
	}

	analysisAddr := strings.TrimSpace(os.Getenv(envAnalysisGRPCAddr))
	if analysisAddr == "" {
		zlogger.Fatal().Msg("ANALYSIS_GRPC_ADDR is required")
	}
	analysisClient, err := analysis.NewClient(analysisAddr, grpcTLSCfg)
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init analysis client")
	}
	marketRepo := repomarket.NewPostgresRepository(qp)
	marketService := appmarket.NewApplication(marketRepo)
	marketHandler := markethandler.NewHandler(marketService)

	analysisService, err := appanalysis.NewApplication(analysisClient, marketRepo)
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init analysis service")
	}

	// --- Live ---

	credKey := []byte(strings.TrimSpace(os.Getenv(envCredentialsKey)))
	if len(credKey) == 0 {
		zlogger.Fatal().Msg("CREDENTIALS_KEY is required")
	}
	encryptor := cryptox.NewEncryptor(credKey)

	nodeRepo := repolive.NewPostgresNodeRepository(qp)
	workerRepo := repolive.NewPostgresWorkerRepository(qp)
	accountRepo := repolive.NewPostgresExchangeAccountRepository(qp, encryptor)

	dockerTLSCfg, err := createDockerTLSConfig()
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init docker tls")
	}

	ghcrUser := strings.TrimSpace(os.Getenv(envGHCRUser))
	ghcrToken := strings.TrimSpace(os.Getenv(envGHCRToken))
	if ghcrToken == "" {
		zlogger.Fatal().Msg("GHCR_TOKEN is required")
	}
	dockerAuthBase64 := base64.StdEncoding.EncodeToString([]byte(ghcrUser + ":" + ghcrToken))

	liveImageURL := strings.TrimSpace(os.Getenv(envLiveImageURL))
	if liveImageURL == "" {
		zlogger.Fatal().Msg("LIVE_IMAGE_URL is required")
	}
	dockerDBImage := strings.TrimSpace(os.Getenv(envDockerDBImage))
	if dockerDBImage == "" {
		dockerDBImage = "timescale/timescaledb:latest-pg16"
	}

	dockerFactory := docker.NewClientFactory(docker.ClientFactoryConfig{
		TLSConfig:  dockerTLSCfg,
		AuthBase64: dockerAuthBase64,
		LiveRefStr: liveImageURL,
		DBRefStr:   dockerDBImage,
	})

	grpcClientFactory, err := grpclive.NewClientFactory(grpcTLSCfg)
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init grpc live client factory")
	}

	grpcCACertPEM, err := os.ReadFile(os.Getenv(envGRPCCAFile))
	if err != nil {
		zlogger.Fatal().Err(err).Msg("read grpc ca cert")
	}
	grpcCAKeyPEM, err := os.ReadFile(os.Getenv(envGRPCCAKeyFile))
	if err != nil {
		zlogger.Fatal().Err(err).Msg("read grpc ca key")
	}
	certGenerator, err := certgen.NewGenerator(grpcCACertPEM, grpcCAKeyPEM)
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init cert generator")
	}

	liveApp, err := applive.NewApplication(applive.ApplicationConfig{
		AppName:             appName,
		WorkerRepo:          workerRepo,
		NodeRepo:            nodeRepo,
		AccountRepo:         accountRepo,
		MarketRepo:          marketRepo,
		TxManager:           txManager,
		EmailSender:         emailSender,
		CertGenerator:       certGenerator,
		DockerFactory:       dockerFactory,
		WorkerClientFactory: grpcClientFactory,
		Logger:              slogAdapter,
	})
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init live app")
	}

	if errs := liveApp.LoadFromDB(ctx); len(errs) > 0 {
		for _, e := range errs {
			zlogger.Warn().Err(e).Msg("live: failed to restore client from db")
		}
	}

	go liveApp.StartExpiringAccountsChecker(ctx)

	// --- Handlers ---

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

	liveHandler, err := livehandler.NewHandler(liveApp)
	if err != nil {
		zlogger.Fatal().Err(err).Msg("init live handler")
	}

	httpRouter, err := router.NewRouter(router.Config{
		AuthHandler:     authHandler,
		MarketHandler:   marketHandler,
		AnalysisHandler: analysisHandler,
		LiveHandler:     liveHandler,
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
	sender := strings.TrimSpace(os.Getenv(envSESSender))
	if sender == "" {
		return nil, fmt.Errorf("SES_SENDER is required")
	}

	sesClient, err := createSESClient(log)
	if err != nil {
		return nil, err
	}

	return awsses.NewClient(sesClient, sender)
}

func createSESClient(log *slog.Logger) (*sesv2.Client, error) {
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

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	baseEndpoint := strings.TrimSpace(os.Getenv(envSESBaseEndpoint))
	if baseEndpoint != "" {
		opts = append(opts, config.WithBaseEndpoint(baseEndpoint))
	}

	opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")))

	if log != nil {
		opts = append(opts, config.WithLogger(awsses.NewAWSLogger(log)))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("create ses client: %w", err)
	}

	return sesv2.NewFromConfig(awsCfg), nil
}

func createDockerTLSConfig() (*tls.Config, error) {
	caCert := []byte(strings.TrimSpace(os.Getenv(envDockerCACert)))
	if len(caCert) == 0 {
		return nil, fmt.Errorf("DOCKER_CA_CERT is required")
	}

	cert := []byte(strings.TrimSpace(os.Getenv(envDockerCert)))
	if len(cert) == 0 {
		return nil, fmt.Errorf("DOCKER_CERT is required")
	}

	key := []byte(strings.TrimSpace(os.Getenv(envDockerKey)))
	if len(key) == 0 {
		return nil, fmt.Errorf("DOCKER_KEY is required")
	}

	tlsCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("docker tls: key pair: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("docker tls: failed to append CA cert")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS13,
	}, nil
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
