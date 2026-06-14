package live

import (
	"errors"
	"log/slog"
	"net"
	"sync"

	"github.com/google/uuid"
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
	accountRepo live.ExchangeAccountRepository
	nodeRepo    live.NodeRepository
	workerRepo  live.WorkerRepository
	marketRepo  domainmarket.Repository

	grpcCACert          string
	certgen             certGenerator
	dockerFactory       *docker.ClientFactory
	workerClientFactory *grpclive.ClientFactory

	clientsMu     sync.RWMutex
	nodeClients   map[uuid.UUID]*docker.Client
	workerClients map[uuid.UUID]*grpclive.Client // accountID -> live.Client

	subsMu      sync.RWMutex                                      // metricsSubs, eventSubs
	metricsSubs map[uuid.UUID]map[uuid.UUID]chan live.Metrics     // accountID -> [subscriberID -> chan]
	statsSubs   map[uuid.UUID]map[uuid.UUID]chan live.WorkerStats // accountID -> [subscriberID -> chan]

	appName     string
	emailSender mailer.Sender
	logger      *slog.Logger
}

type ApplicationConfig struct {
	AccountRepo live.ExchangeAccountRepository
	NodeRepo    live.NodeRepository
	WorkerRepo  live.WorkerRepository
	MarketRepo  domainmarket.Repository

	GRPCCACert          string
	CertGenerator       certGenerator
	DockerClientFactory *docker.ClientFactory
	WorkerClientFactory *grpclive.ClientFactory

	EmailSender mailer.Sender
	AppName     string
	Logger      *slog.Logger
}

func NewApplication(cfg ApplicationConfig) (*Application, error) {
	if cfg.AccountRepo == nil {
		return nil, errors.New("live app: account repository is nil")
	}
	if cfg.NodeRepo == nil {
		return nil, errors.New("live app: node repository is nil")
	}
	if cfg.WorkerRepo == nil {
		return nil, errors.New("live app: worker repository is nil")
	}
	if cfg.MarketRepo == nil {
		return nil, errors.New("live app: market repository is nil")
	}

	if cfg.GRPCCACert == "" {
		return nil, errors.New("live app: grpc ca cert is empty")
	}
	if cfg.CertGenerator == nil {
		return nil, errors.New("live app: cert generator is nil")
	}
	if cfg.DockerClientFactory == nil {
		return nil, errors.New("live app: docker factory is nil")
	}
	if cfg.WorkerClientFactory == nil {
		return nil, errors.New("live app: worker client factory is nil")
	}

	if cfg.EmailSender == nil {
		return nil, errors.New("live app: worker repository is nil")
	}

	logger := slog.Default()
	if cfg.Logger != nil {
		logger = cfg.Logger
	}
	return &Application{
		accountRepo: cfg.AccountRepo,
		nodeRepo:    cfg.NodeRepo,
		workerRepo:  cfg.WorkerRepo,
		marketRepo:  cfg.MarketRepo,

		grpcCACert:          cfg.GRPCCACert,
		certgen:             cfg.CertGenerator,
		dockerFactory:       cfg.DockerClientFactory,
		workerClientFactory: cfg.WorkerClientFactory,

		nodeClients:   make(map[uuid.UUID]*docker.Client),
		workerClients: make(map[uuid.UUID]*grpclive.Client),

		metricsSubs: make(map[uuid.UUID]map[uuid.UUID]chan live.Metrics),
		statsSubs:   make(map[uuid.UUID]map[uuid.UUID]chan live.WorkerStats),

		emailSender: cfg.EmailSender,
		appName:     cfg.AppName,
		logger:      logger,
	}, nil
}
