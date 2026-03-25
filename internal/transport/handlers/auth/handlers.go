package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/lib/errorsx"
	"github.com/pulsoats/core/lib/logx"
	"github.com/pulsoats/main/internal/domain/auth"
	"github.com/pulsoats/main/internal/transport/errorx"
	"github.com/pulsoats/main/internal/transport/middleware"
)

type service interface {
	InviteToken(ctx context.Context, userID int64) (string, error)

	Register(ctx context.Context, email, password string, inviteToken string) error
	VerifyEmailByToken(ctx context.Context, emailVerificationToken string) error

	Login(ctx context.Context, input auth.LoginInput) (resp auth.LoginResponse, err error)
	Logout(ctx context.Context, sessionID int64) error
	LogoutAll(ctx context.Context, userID int64, exceptedSessionID int64) error

	RefreshToken(ctx context.Context, currentToken string) (auth.LoginResponse, error)

	ChangePassword(ctx context.Context, input auth.ChangePasswordInput) error
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, resetPasswordToken string, newPassword string) error
	EnsureRoot(ctx context.Context, email, password string) error
}

type Handler struct {
	service service
	baseURL string
	logger  logx.Logger
}

type Config struct {
	Service service
	BaseURL string
	Logger  logx.Logger
}

func NewHandler(cfg Config) (*Handler, error) {
	if cfg.Service == nil {
		return nil, fmt.Errorf("auth handler: %w: auth service", derrors.ErrRequired)
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("auth handler: %w: base url", derrors.ErrRequired)
	}
	if cfg.Logger == nil {
		return nil, fmt.Errorf("auth handler: %w: logger", derrors.ErrRequired)
	}
	return &Handler{
		service: cfg.Service,
		baseURL: cfg.BaseURL,
		logger:  cfg.Logger,
	}, nil
}

func (h *Handler) CreateInviteToken(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		errorx.RespondError(c, derrors.ErrUnauthorized)
		return
	}

	token, err := h.service.InviteToken(c.Request.Context(), userID)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	u, err := url.Parse(h.baseURL)
	if err != nil {
		h.logger.Error("")
		errorx.RespondError(c, errorsx.ErrInternal)
		return
	}

	u = u.JoinPath("register")
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()

	c.JSON(http.StatusCreated, createInviteTokenResponse{
		InviteToken: token,
		InviteLink:  u.String(),
	})
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: %s", derrors.ErrInvalidArgument, err.Error()))
		return
	}

	err := h.service.Register(c.Request.Context(), req.Email, req.Password, req.InviteToken)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.Status(http.StatusCreated)
}

func (h *Handler) VerifyEmail(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		errorx.RespondError(c, fmt.Errorf("%w: token", derrors.ErrRequired))
		return
	}

	if err := h.service.VerifyEmailByToken(c.Request.Context(), token); err != nil {
		errorx.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: %s", derrors.ErrInvalidArgument, err.Error()))
		return
	}

	ip := c.ClientIP()
	agent := c.Request.UserAgent()

	input := auth.LoginInput{
		Email:     req.Email,
		Password:  req.Password,
		IPAddress: &ip,
		UserAgent: &agent,
	}

	resp, err := h.service.Login(c.Request.Context(), input)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, loginResponse{AccessToken: resp.AccessToken, RefreshToken: resp.RefreshToken})
}

func (h *Handler) Logout(c *gin.Context) {
	sessionID, ok := middleware.GetSessionID(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	err := h.service.Logout(c.Request.Context(), sessionID)
	if err != nil {
		errorx.RespondError(c, err)
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) LogoutAll(c *gin.Context) {
	sessionID, ok := middleware.GetSessionID(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.Status(http.StatusUnauthorized)
		return
	}

	err := h.service.LogoutAll(c.Request.Context(), userID, sessionID)
	if err != nil {
		errorx.RespondError(c, err)
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) RefreshToken(c *gin.Context) {
	var req refreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: %s", derrors.ErrInvalidArgument, err.Error()))
		return
	}

	token := req.RefreshToken

	resp, err := h.service.RefreshToken(c.Request.Context(), token)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, refreshTokenResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	})
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: %s", derrors.ErrInvalidArgument, err.Error()))
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		errorx.RespondError(c, derrors.ErrUnauthorized)
		return
	}
	sessionID, ok := middleware.GetSessionID(c)
	if !ok {
		errorx.RespondError(c, derrors.ErrUnauthorized)
		return
	}

	err := h.service.ChangePassword(c.Request.Context(), auth.ChangePasswordInput{
		UserID:           userID,
		CurrentSessionID: sessionID,
		CurrentPassword:  req.CurrentPassword,
		NewPassword:      req.NewPassword,
	})
	if err != nil {
		errorx.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password changed"})
}

func (h *Handler) RequestPasswordReset(c *gin.Context) {
	var req passwordResetEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorx.RespondError(c, err)
		return
	}

	err := h.service.RequestPasswordReset(c.Request.Context(), req.Email)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset email sent"})
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorx.RespondError(c, err)
		return
	}

	err := h.service.ResetPassword(c.Request.Context(), req.ResetPasswordToken, req.NewPassword)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password has been reset"})
}
