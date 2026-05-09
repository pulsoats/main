package core

import (
	"encoding/json"

	"github.com/google/uuid"
)

type MarketSpecRequest struct {
	Exchange string `json:"exchange" binding:"required"`
	Category string `json:"category" binding:"required"`
	Symbol   string `json:"symbol" binding:"required"`
}

type MarketSpecResponse struct {
	Exchange string `json:"exchange"`
	Category string `json:"category"`
	Symbol   string `json:"symbol"`
}

type DetectorConfigRequest struct {
	Code      string          `json:"code" binding:"required"`
	OptsLabel string          `json:"optsLabel"`
	Opts      json.RawMessage `json:"opts"`
}

type DetectorConfigResponse struct {
	Code      string          `json:"code"`
	OptsLabel string          `json:"optsLabel"`
	Opts      json.RawMessage `json:"opts"`
}

type DetectorMetaResponse struct {
	Code        string          `json:"code"`
	Kind        string          `json:"kind"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	OptsSchema  json.RawMessage `json:"optsSchema"`
}

type ListAvailableDetectorsResponse struct {
	Detectors []DetectorMetaResponse `json:"detectors"`
}

type ExchangeMetaResponse struct {
	Code       string   `json:"code"`
	Categories []string `json:"categories"`
	Intervals  []string `json:"intervals"`
}

type ListAvailableExchangesResponse struct {
	Exchanges []ExchangeMetaResponse `json:"exchanges"`
}

type RunStatusResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

type BaseRunResponse struct {
	ID              uuid.UUID              `json:"id"`
	Status          RunStatusResponse      `json:"status"`
	Market          MarketSpecResponse     `json:"market"`
	Interval        string                 `json:"interval"`
	FirstCandleTime string                 `json:"firstCandleTime"` // RFC3339
	LastCandleTime  string                 `json:"lastCandleTime"`  // RFC3339
	Detector        DetectorConfigResponse `json:"detector"`
	SignalsCount    int                    `json:"signalsCount"`
	CreatedBy       string                 `json:"createdBy"`
	CreatedAt       string                 `json:"createdAt"`
}

type ServiceInfoResponse struct {
	ID         uuid.UUID `json:"id"`
	Kind       string    `json:"kind"`
	Addr       string    `json:"address"`
	Name       string    `json:"name"`
	Exchange   string    `json:"exchange"`
	Account    string    `json:"account"`
	Version    string    `json:"version"`
	LastSeenAt string    `json:"lastSeenAt"`
	CreatedAt  string    `json:"createdAt"`
}

type ServiceMetricsResponse struct {
	Status        string  `json:"status"`
	CpuPercent    float64 `json:"cpuPercent"`
	MemoryPercent float64 `json:"memoryPercent"`
	UptimeSeconds string  `json:"uptimeSeconds"`
	ReportedAt    string  `json:"reportedAt"`
}
