package errorx

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/pulsoats/core/errorsx"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func RespondError(c *gin.Context, err error) {
	c.Error(err) // store for middleware logging
	status, apiErr := MapError(err)
	c.AbortWithStatusJSON(status, apiErr)
}

func MapError(err error) (int, APIError) {
	switch {
	case errors.Is(err, errorsx.ErrNotFound):
		return http.StatusNotFound, APIError{"not_found", friendlyMessage(err)}
	case errors.Is(err, errorsx.ErrUnauthorized), errors.Is(err, errorsx.ErrUnauthorized):
		return http.StatusUnauthorized, APIError{"unauthorized", "Unauthorized"}
	case errors.Is(err, errorsx.ErrAlreadyExists):
		return http.StatusConflict, APIError{"conflict", friendlyMessage(err)}
	case errors.Is(err, errorsx.ErrInvalidArgument), errors.Is(err, errorsx.ErrRequired):
		return http.StatusUnprocessableEntity, APIError{"invalid_input", friendlyMessage(err)}
	case errors.Is(err, errorsx.ErrInternal):
		return http.StatusInternalServerError, APIError{"internal", "Internal server error"}
	default:
		return http.StatusBadRequest, APIError{"bad_request", friendlyMessage(err)}
	}
}

func friendlyMessage(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		return validationMessage(ve)
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "invite token expired"):
		return "Invite token expired"
	case strings.Contains(msg, "invite token used"):
		return "Invite token already used"
	case strings.Contains(msg, "invalid credentials"):
		return "Invalid credentials"
	case strings.Contains(msg, "admin check"):
		return "Admin access required"
	default:
		return msg
	}
}

func validationMessage(ve validator.ValidationErrors) string {
	if len(ve) == 0 {
		return "Invalid input"
	}

	messages := make([]string, 0, len(ve))
	for _, fe := range ve {
		field := strings.ToLower(fe.Field())
		switch fe.Tag() {
		case "required":
			messages = append(messages, fmt.Sprintf("%s is required", field))
		case "email":
			messages = append(messages, fmt.Sprintf("%s must be a valid email", field))
		case "min":
			messages = append(messages, fmt.Sprintf("%s must be at least %s characters", field, fe.Param()))
		default:
			messages = append(messages, fmt.Sprintf("%s is invalid", field))
		}
	}
	return strings.Join(messages, ", ")
}
