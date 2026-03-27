package detectors

import (
	"fmt"

	"github.com/pulsoats/core/domain/detect"
	"github.com/pulsoats/core/domain/detect/detectors"
	"github.com/pulsoats/core/errorsx"
)

type Application struct {
	reg *detectors.Registry
}

type ApplicationConfig struct {
	DetectorsRegistry *detectors.Registry
}

func NewApplication(cfg ApplicationConfig) (*Application, error) {
	if cfg.DetectorsRegistry == nil {
		return nil, fmt.Errorf("detectors app: %w", errorsx.ErrInvalidArgument)
	}
	return &Application{reg: cfg.DetectorsRegistry}, nil
}

func (s *Application) ListMetas() []detect.DetectorMeta {
	return s.reg.ListMetas()
}
