package detectors

import (
	"fmt"

	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/domain/detect"
	coredetectors "github.com/pulsoats/core/domain/detect/detectors"
	"github.com/pulsoats/main/internal/domain/detectors"
)

type service struct {
	reg *coredetectors.Registry
}

type ServiceConfig struct {
	DetectorsRegistry *coredetectors.Registry
}

func NewService(cfg ServiceConfig) (detectors.Service, error) {
	if cfg.DetectorsRegistry == nil {
		return nil, fmt.Errorf("%w: detectors reg", derrors.ErrRequired)
	}
	return &service{reg: cfg.DetectorsRegistry}, nil
}

func (s *service) ListMetas() []detect.DetectorMeta {
	return s.reg.ListMetas()
}
