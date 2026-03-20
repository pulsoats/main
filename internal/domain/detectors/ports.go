package detectors

import "github.com/pulsoats/core/domain/detect"

type Service interface {
	ListMetas() []detect.DetectorMeta
}
