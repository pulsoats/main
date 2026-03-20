package market

import "github.com/pulsoats/core/domain/exchange"

type listExchangeMetasResponse struct {
	Metas []exchangeMeta `json:"metas"`
}

type exchangeMeta struct {
	Code       string   `json:"exchange_code"`
	Intervals  []string `json:"intervals"`
	Categories []string `json:"categories"`
	PriceTypes []string `json:"price_types"`
}

func mapExchangeMetasToResponse(metas []exchange.Meta) listExchangeMetasResponse {
	res := make([]exchangeMeta, 0, len(metas))
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

		res = append(res, exchangeMeta{
			Code:       m.Code,
			Intervals:  intervals,
			Categories: categories,
			PriceTypes: priceTypes,
		})
	}

	return listExchangeMetasResponse{Metas: res}
}

type listSymbolsResponse struct {
	Symbols []string `json:"symbols"`
}
