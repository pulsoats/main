package health

import (
	"errors"

	"github.com/google/uuid"
	systempb "github.com/pulsoats/contracts/gen/go/system/v1"
	"github.com/pulsoats/core/system"
)

func ServiceInfoFromProto(pb *systempb.ServiceInfo) (system.ServiceInfo, error) {
	if pb == nil {
		return system.ServiceInfo{}, errors.New("resp is nil")
	}

	id, err := uuid.Parse(pb.Id)
	if err != nil {
		return system.ServiceInfo{}, errors.New("invalid id")
	}

	return system.ServiceInfo{
		ID:       id,
		Kind:     system.ServiceKind(pb.Kind),
		Name:     pb.Name,
		Exchange: pb.Exchange,
		Account:  pb.Account,
		Version:  pb.Version,
	}, nil
}

func ServiceMetricsFromProto(pb *systempb.ServiceMetrics) (system.ServiceMetrics, error) {
	if pb == nil {
		return system.ServiceMetrics{}, errors.New("resp is nil")
	}

	id, err := uuid.Parse(pb.ServiceId)
	if err != nil {
		return system.ServiceMetrics{}, errors.New("invalid service id")
	}

	return system.ServiceMetrics{
		ServiceID:     id,
		Status:        system.ServiceStatus(pb.Status),
		CpuPercent:    pb.CpuPercent,
		MemoryPercent: pb.MemoryPercent,
		UptimeSeconds: pb.UptimeSeconds,
		ReportedAt:    pb.ReportedAt.AsTime(),
	}, nil
}
