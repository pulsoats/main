package market

import (
	"github.com/pulsoats/main/internal/domain/market"
	"github.com/pulsoats/main/internal/transport/handlers/core"
)

type suggestResponse struct {
	Suggestions []core.MarketSpecResponse `json:"suggestions"`
}

func suggestionsToResponse(suggestions []market.Suggestion) suggestResponse {
	res := make([]core.MarketSpecResponse, 0, len(suggestions))
	for _, s := range suggestions {
		res = append(res, core.MarketSpecResponse{
			Exchange: s.Exchange,
			Category: s.Category,
			Symbol:   s.Symbol,
		})
	}
	return suggestResponse{Suggestions: res}
}
