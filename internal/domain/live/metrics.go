package live

import "github.com/google/uuid"

type Metrics struct {
	WorkerID    uuid.UUID
	ContainerID string
	CPUPercent  float64
	MemoryBytes uint64
}
