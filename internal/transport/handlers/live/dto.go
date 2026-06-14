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
	Name       string  `json:"name" binding:"required"`
	Exchange   string  `json:"exchange"   binding:"required"`
	Host       string  `json:"host"       binding:"required"`
	DockerPort int     `json:"dockerPort" binding:"required"`
	Region     string  `json:"region"     binding:"required"`
	MaxWorkers int     `json:"maxWorkers" binding:"required"`
	DSN        *string `json:"dsn"`
	DBUser     string  `json:"dbUser"`
	DBPassword string  `json:"dbPassword"`
}

type nodeResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
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

type runResponse struct {
	core.BaseRunResponse
	SumProfitPercent float64 `json:"sumProfitPercent"`
	FinishedAt       *string `json:"finishedAt,omitempty"`
	FinishedBy       *string `json:"finishedBy,omitempty"`
}

type newRunRequest struct {
	Market   core.MarketSpecRequest     `json:"market" binding:"required"`
	Interval string                     `json:"interval" binding:"required"`
	Detector core.DetectorConfigRequest `json:"detector" binding:"required"`
}

type runsPagedRequest struct {
	Limit       int32  `form:"limit"`
	BeforeID    string `form:"before_id"`
	OrderDirAsc bool   `form:"order_dir_asc"`

	Categories    []string `form:"categories"`
	Symbols       []string `form:"symbols"`
	Intervals     []string `form:"intervals"`
	DetectorCodes []string `form:"detector_codes"`
	Statuses      []int    `form:"statuses"`

	MinSignals *int64 `form:"min_signals"`
	MaxSignals *int64 `form:"max_signals"`

	CreatedFrom *time.Time `form:"created_from"`
	CreatedTo   *time.Time `form:"created_to"`
}

type runsPagedResponse struct {
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
	CandleTime        int64                   `json:"candleTime"`
	CandleValue       int64                   `json:"candleValue"`
	BuyValue          int64                   `json:"buyValue"`
	TakeProfitValue   int64                   `json:"takeProfitValue"`
	StopLossValue     int64                   `json:"stopLossValue"`
	ExpectedReturnPPM int64                   `json:"expectedReturnPpm"`
	Fingerprint       uuid.UUID               `json:"fingerprint"`
	CreatedAt         int64                   `json:"createdAt"`
}

type signalsPagedRequest struct {
	Limit       int32  `form:"limit"`
	BeforeID    string `form:"before_id"`
	OrderDirAsc bool   `form:"order_dir_asc"`
	RunID       string `form:"run_id"`

	Categories    []string `form:"categories"`
	Symbols       []string `form:"symbols"`
	Intervals     []string `form:"intervals"`
	DetectorCodes []string `form:"detector_codes"`

	CreatedFrom *time.Time `form:"created_from"`
	CreatedTo   *time.Time `form:"created_to"`
}

type signalsPagedResponse struct {
	Signals      []signalResponse `json:"signals"`
	NextBeforeID *uuid.UUID       `json:"nextBeforeId,omitempty"`
	HasMore      bool             `json:"hasMore"`
}

type workerStatsResponse struct {
	RunsTotal    int32 `json:"runsTotal"`
	SignalsTotal int64 `json:"signalsTotal"`
}
