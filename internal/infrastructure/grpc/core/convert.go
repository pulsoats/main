package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TimeFromProto конвертирует опциональный pb-timestamp в value time.Time.
// Отсутствие значения (nil) — легальное состояние и трактуется как zero time:
// домен моделирует "значения нет" нулевым временем, а не указателем.
func TimeFromProto(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

// TimePtrFromProto конвертирует опциональный pb-timestamp в *time.Time: nil → nil.
func TimePtrFromProto(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}

// TimePtrToProto конвертирует *time.Time в pb-timestamp: nil → nil.
func TimePtrToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// RequireTime извлекает обязательный pb-timestamp, мапящийся в value-поле домена.
// nil = непредвиденное поведение доверенного сервиса → errorsx.ErrInternal.
func RequireTime(op, field string, ts *timestamppb.Timestamp) (time.Time, error) {
	if ts == nil {
		return time.Time{}, fmt.Errorf("%s: %s is nil: %w", op, field, errorsx.ErrInternal)
	}
	return ts.AsTime(), nil
}

// UUIDPtrToProto сериализует опциональный uuid в строковый указатель: nil → nil.
func UUIDPtrToProto(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

// UUIDPtrFromProto разбирает опциональный строковый uuid (напр. next_before_id): nil → nil.
// Непарсимое значение от доверенного сервиса → errorsx.ErrInternal.
func UUIDPtrFromProto(op, field string, raw *string) (*uuid.UUID, error) {
	if raw == nil {
		return nil, nil
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid %s %q: %w", op, field, *raw, errors.Join(errorsx.ErrInternal, err))
	}
	return &id, nil
}
