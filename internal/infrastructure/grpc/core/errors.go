package core

import (
	"errors"
	"fmt"

	"github.com/pulsoats/core/errorsx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func MapError(err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("%w", errors.Join(errorsx.ErrInternal, err))
	}

	switch st.Code() {
	case codes.NotFound:
		return fmt.Errorf("%w: %s", errorsx.ErrNotFound, st.Message())
	case codes.AlreadyExists:
		return fmt.Errorf("%w: %s", errorsx.ErrAlreadyExists, st.Message())
	case codes.InvalidArgument, codes.OutOfRange, codes.FailedPrecondition:
		return fmt.Errorf("%w: %s", errorsx.ErrInvalidArgument, st.Message())
	case codes.Unauthenticated:
		return fmt.Errorf("%w: %s", errorsx.ErrUnauthorized, st.Message())
	case codes.PermissionDenied:
		return fmt.Errorf("%w: %s", errorsx.ErrForbidden, st.Message())
	case codes.Unavailable:
		return fmt.Errorf("%w: %s", errorsx.ErrInternal, st.Message())
	default:
		return fmt.Errorf("%w", errors.Join(errorsx.ErrInternal, err))
	}
}
