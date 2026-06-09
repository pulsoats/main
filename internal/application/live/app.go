package live

import (
	"errors"
	"log/slog"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/pulsoats/main/internal/domain"
	"github.com/pulsoats/main/internal/domain/live"
	"github.com/pulsoats/main/internal/domain/mailer"
	domainmarket "github.com/pulsoats/main/internal/domain/market"
	"github.com/pulsoats/main/internal/infrastructure/docker"
	grpclive "github.com/pulsoats/main/internal/infrastructure/grpc/live"
)

const dbContainerName = "%s-live-db"

type certGenerator interface {
	GenerateServerCert(ip net.IP) (certPEM, keyPEM []byte, err error)
}

type Application struct {
	workerRepo          live.WorkerRepository
	nodeRepo            live.NodeRepository
	accountRepo         live.ExchangeAccountRepository
	marketRepo          domainmarket.Repository
	txManager           domain.TxManager
	emailSender         mailer.Sender
	certgen             certGenerator
	dockerFactory       *docker.ClientFactory
	workerClientFactory *grpclive.ClientFactory

	mu            sync.RWMutex
	nodeClients   map[uuid.UUID]*docker.Client
	workerClients map[uuid.UUID]*grpclive.Client                // accountID -> live.Client
	metricsSubs   map[uuid.UUID]map[uuid.UUID]chan live.Metrics // accountID -> [subscriberID -> chan]

	grpcCACert string
	appName    string
	logger     *slog.Logger
}

type ApplicationConfig struct {
	AppName       string
	WorkerRepo    live.WorkerRepository
	NodeRepo      live.NodeRepository
	AccountRepo   live.ExchangeAccountRepository
	MarketRepo    domainmarket.Repository
	TxManager     domain.TxManager
	EmailSender   mailer.Sender
	CertGenerator certGenerator

	DockerFactory       *docker.ClientFactory
	WorkerClientFactory *grpclive.ClientFactory
	Logger              *slog.Logger
}

func NewApplication(cfg ApplicationConfig) (*Application, error) {
	if cfg.WorkerRepo == nil {
		return nil, errors.New("live app: worker repository is nil")
	}
	if cfg.NodeRepo == nil {
		return nil, errors.New("live app: node repository is nil")
	}
	if cfg.AccountRepo == nil {
		return nil, errors.New("live app: account repository is nil")
	}
	if cfg.MarketRepo == nil {
		return nil, errors.New("live app: market repository is nil")
	}
	if cfg.TxManager == nil {
		return nil, errors.New("live app: tx (transaction) manager is nil")
	}
	if cfg.EmailSender == nil {
		return nil, errors.New("live app: worker repository is nil")
	}
	if cfg.DockerFactory == nil {
		return nil, errors.New("live app: docker factory is nil")
	}
	if cfg.CertGenerator == nil {
		return nil, errors.New("live app: cert generator is nil")
	}

	logger := slog.Default()
	if cfg.Logger != nil {
		logger = cfg.Logger
	}
	return &Application{
		appName:       cfg.AppName,
		workerRepo:    cfg.WorkerRepo,
		nodeRepo:      cfg.NodeRepo,
		accountRepo:   cfg.AccountRepo,
		marketRepo:    cfg.MarketRepo,
		txManager:     cfg.TxManager,
		emailSender:   cfg.EmailSender,
		certgen:       cfg.CertGenerator,
		dockerFactory: cfg.DockerFactory,
		nodeClients:   make(map[uuid.UUID]*docker.Client),
		workerClients: make(map[uuid.UUID]*grpclive.Client),
		metricsSubs:   make(map[uuid.UUID]map[uuid.UUID]chan live.Metrics),
		logger:        logger,
	}, nil
}
