package detectors

import (
	"encoding/json"

	"github.com/pulsoats/core/domain/detect"
)

type detectorMeta struct {
	Code        string          `json:"code"`
	Description string          `json:"description"`
	Kind        string          `json:"kind"`
	OptsSchema  json.RawMessage `json:"opts_schema"`
}

type listDetectorsResponse struct {
	Detectors []detectorMeta `json:"detectors"`
}

func mapMetasToResponse(metas []detect.DetectorMeta) listDetectorsResponse {
	detectors := make([]detectorMeta, 0, len(metas))
	for _, m := range metas {
		detectors = append(detectors, detectorMeta{
			Code:        m.Code,
			Description: m.Desc,
			Kind:        string(m.Kind),
			OptsSchema:  m.OptsSchema,
		})
	}
	return listDetectorsResponse{Detectors: detectors}
}
