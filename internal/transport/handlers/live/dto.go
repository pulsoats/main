package live

import (
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/main/internal/transport/handlers/core"
)

type createExchangeAccountCredentialsRequest struct {
	APIKey     string `json:"apiKey" binding:"required"`
	APISecret  string `json:"apiSecret"`
	Passphrase string `json:"passphrase"`
}

type createAccountRequest struct {
	Exchange    string                                  `json:"exchange" binding:"required"`
	Name        string                                  `json:"name" binding:"required"`
	Credentials createExchangeAccountCredentialsRequest `json:"credentials" binding:"required"`
	ExpiresAt   time.Time                               `json:"expiresAt" binding:"required"`
}

type exchangeAccountResponse struct {
	ID        uuid.UUID `json:"id"`
	Exchange  string    `json:"exchange"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	ExpiresAt string    `json:"expiresAt"`
	CreatedAt string    `json:"createdAt"`
	UpdatedAt string    `json:"updatedAt"`
}

type exchangeAccountsResponse struct {
	Accounts []exchangeAccountResponse `json:"accounts"`
}

type updateExchangeAccountCredentialsRequest struct {
	ExchangeAccountID uuid.UUID                               `json:"accountId" binding:"required"`
	Credentials       createExchangeAccountCredentialsRequest `json:"credentials" binding:"required"`
	ExpiresAt         time.Time                               `json:"expiresAt" binding:"required"`
}

type createNodeRequest struct {
	Exchange   string `json:"exchange" binding:"required"`
	Host       string `json:"host" binding:"required"`
	DockerPort int    `json:"dockerPort" binding:"required"`
	Region     string `json:"region" binding:"required"`
	MaxWorkers int    `json:"maxWorkers" binding:"required"`
	DBUser     string `json:"dbUser" binding:"required"`
	DBPassword string `json:"dbPassword" binding:"required"`
}

type nodeResponse struct {
	ID           uuid.UUID `json:"id"`
	Exchange     string    `json:"exchange"`
	Host         string    `json:"host"`
	DockerPort   int       `json:"dockerPort"`
	Region       string    `json:"region"`
	MaxWorkers   int       `json:"maxWorkers"`
	WorkersCount int       `json:"workersCount"`
	Status       string    `json:"status"`
	LastError    *string   `json:"lastError"`
	CreatedAt    string    `json:"createdAt"` //RFC3339
}

type nodesResponse struct {
	Nodes []nodeResponse `json:"nodes"`
}

type createWorkerRequest struct {
	ExchangeAccountID uuid.UUID
}

type workerResponse struct {
	ID                uuid.UUID `json:"id"`
	NodeID            uuid.UUID `json:"nodeId"`
	Host              string    `json:"host"`
	GRPCPort          int       `json:"grpcPort"`
	ContainerID       string    `json:"containerId"`
	Name              string    `json:"name"`
	ExchangeAccountID uuid.UUID `json:"exchangeAccountId"`
	Status            string    `json:"status"`
	LastError         *string   `json:"lastError"`
	CreatedAt         string    `json:"createdAt"`
}

type workersResponse struct {
	Workers []workerResponse `json:"workers"`
}

type newRunRequest struct {
	Market   core.MarketSpecRequest     `json:"market" binding:"required"`
	Interval string                     `json:"interval" binding:"required"`
	Detector core.DetectorConfigRequest `json:"detector" binding:"required"`
}

type listRunsQuery struct {
	Limit    int32      `form:"limit"`
	BeforeID *uuid.UUID `form:"beforeId"`
}

type listSignalsQuery struct {
	Limit    int32      `form:"limit"`
	BeforeID *uuid.UUID `form:"beforeId"`
	RunID    *uuid.UUID `form:"runId"`
}

type runResponse struct {
	core.BaseRunResponse
	OrdersCount      int64   `json:"ordersCount"`
	SumProfitPercent float64 `json:"sumProfitPercent"`
	FinishedAt       *string `json:"finishedAt,omitempty"`
	FinishedBy       *string `json:"finishedBy,omitempty"`
}

type listRunsResponse struct {
	Runs         []runResponse `json:"runs"`
	NextBeforeID *uuid.UUID    `json:"nextBeforeId,omitempty"`
	HasMore      bool          `json:"hasMore"`
}

type signalResponse struct {
	ID                uuid.UUID               `json:"id"`
	RunID             uuid.UUID               `json:"runId"`
	Market            core.MarketSpecResponse `json:"market"`
	DetectorCode      string                  `json:"detectorCode"`
	DetectorOptsLabel string                  `json:"detectorOptsLabel"`
	Time              int64                   `json:"time"`
	Value             int64                   `json:"value"`
	BuyValue          int64                   `json:"buyValue"`
	TakeProfitValue   int64                   `json:"takeProfitValue"`
	StopLossValue     int64                   `json:"stopLossValue"`
	ExpectedReturnPPM int64                   `json:"expectedReturnPpm"`
	Fingerprint       uuid.UUID               `json:"fingerprint"`
	CreatedAt         int64                   `json:"createdAt"`
}

type listSignalsResponse struct {
	Signals      []signalResponse `json:"signals"`
	NextBeforeID *uuid.UUID       `json:"nextBeforeId,omitempty"`
	HasMore      bool             `json:"hasMore"`
}
