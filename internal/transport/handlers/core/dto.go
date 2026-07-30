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
	Version   string          `json:"version"`
	OptsLabel string          `json:"optsLabel"`
	Opts      json.RawMessage `json:"opts"`
}

type DetectorConfigResponse struct {
	Code      string          `json:"code"`
	Version   string          `json:"version"`
	OptsLabel string          `json:"optsLabel"`
	Opts      json.RawMessage `json:"opts"`
}

type DetectorMetaResponse struct {
	Code        string          `json:"code"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	OptsSchema  json.RawMessage `json:"optsSchema"`
}

type AvailableDetectorsResponse struct {
	Detectors []DetectorMetaResponse `json:"detectors"`
}

type FilterConfigRequest struct {
	Code   string `json:"code" binding:"required"`
	Period int    `json:"period"`
}

type FilterConfigResponse struct {
	Code   string `json:"code"`
	Period int    `json:"period"`
}

type FilterMetaResponse struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type AvailableFiltersResponse struct {
	Filters []FilterMetaResponse `json:"filters"`
}

type ExchangeMetaResponse struct {
	Code       string   `json:"code"`
	Categories []string `json:"categories"`
	Intervals  []string `json:"intervals"`
}

type AvailableExchangesResponse struct {
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
	DetectorConfig  DetectorConfigResponse `json:"detectorConfig"`
	FiltersConfigs  []FilterConfigResponse `json:"filtersConfigs"`
	SignalsCount    int                    `json:"signalsCount"`
	CreatedBy       string                 `json:"createdBy"`
	CreatedAt       string                 `json:"createdAt"`
}
