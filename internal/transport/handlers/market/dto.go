package market

import "github.com/pulsoats/core/domain/exchange"

type exchangeMetaResponse struct {
	Code       string   `json:"code"`
	Intervals  []string `json:"intervals"`
	Categories []string `json:"categories"`
	PriceTypes []string `json:"priceTypes"`
}

func mapExchangeMetasToResponseSlice(metas []exchange.Meta) []exchangeMetaResponse {
	res := make([]exchangeMetaResponse, 0, len(metas))
	for _, m := range metas {
		intervals := make([]string, 0, len(m.Intervals))
		for _, i := range m.Intervals {
			intervals = append(intervals, i.String())
		}

		categories := make([]string, 0, len(m.Categories))
		for _, c := range m.Categories {
			categories = append(categories, string(c))
		}

		priceTypes := make([]string, 0, len(m.PriceTypes))
		for _, t := range m.PriceTypes {
			priceTypes = append(priceTypes, string(t))
		}

		res = append(res, exchangeMetaResponse{
			Code:       m.Code,
			Intervals:  intervals,
			Categories: categories,
			PriceTypes: priceTypes,
		})
	}

	return res
}
