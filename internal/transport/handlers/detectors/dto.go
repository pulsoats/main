package detectors

import (
	"encoding/json"

	"github.com/pulsoats/core/domain/detect"
)

type detectorMetaResponse struct {
	Code        string          `json:"code"`
	Description string          `json:"description"`
	Kind        string          `json:"kind"`
	OptsSchema  json.RawMessage `json:"optsSchema"`
}

func mapMetasToResponseSlice(metas []detect.DetectorMeta) []detectorMetaResponse {
	res := make([]detectorMetaResponse, 0, len(metas))
	for _, m := range metas {
		res = append(res, detectorMetaResponse{
			Code:        m.Code,
			Description: m.Desc,
			Kind:        string(m.Kind),
			OptsSchema:  m.OptsSchema,
		})
	}
	return res
}
