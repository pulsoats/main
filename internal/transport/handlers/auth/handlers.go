package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/lib/logx"
	"github.com/pulsoats/main/internal/domain/auth"
	"github.com/pulsoats/main/internal/transport/errorx"
	"github.com/pulsoats/main/internal/transport/middleware"
)

type Handler struct {
	app    app
	logger logx.Logger
}

type Config struct {
	Application app
	BaseURL     string
	Logger      logx.Logger
}

func NewHandler(cfg Config) (*Handler, error) {
	if cfg.Application == nil {
		return nil, fmt.Errorf("auth handler: %w: auth service", errorsx.ErrRequired)
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("auth handler: %w: base url", errorsx.ErrRequired)
	}
	if cfg.Logger == nil {
		return nil, fmt.Errorf("auth handler: %w: logger", errorsx.ErrRequired)
	}
	return &Handler{
		app:    cfg.Application,
		logger: cfg.Logger,
	}, nil
}

func (h *Handler) CreateInviteToken(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		errorx.RespondError(c, errors.Join(errorsx.ErrInternal, errors.New("gin cotext: wrong user id")))
		return
	}
	role, ok := middleware.GetRole(c)
	if !ok {
		errorx.RespondError(c, errors.Join(errorsx.ErrInternal, errors.New("gin context: wrong role")))
		return
	}

	token, link, err := h.app.CreateInviteToken(c.Request.Context(), userID, role)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, createInviteTokenResponse{
		Token: mapToInviteTokenResponse(token),
		Link:  link,
	})
}

func (h *Handler) ListInviteTokens(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		errorx.RespondError(c, errors.Join(errorsx.ErrInternal, errors.New("gin cotext: wrong user id")))
		return
	}
	role, ok := middleware.GetRole(c)
	if !ok {
		errorx.RespondError(c, errors.Join(errorsx.ErrInternal, errors.New("gin context: wrong role")))
		return
	}

	tokens, err := h.app.ListInviteTokens(c.Request.Context(), userID, role)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	resp := mapInviteTokensToSliceResponse(tokens)
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) RevokeInviteToken(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		errorx.RespondError(c, errors.Join(errorsx.ErrInternal, errors.New("gin cotext: wrong user id")))
		return
	}
	role, ok := middleware.GetRole(c)
	if !ok {
		errorx.RespondError(c, errors.Join(errorsx.ErrInternal, errors.New("gin context: wrong role")))
		return
	}

	rawID := strings.TrimSpace(c.Param("token_id"))
	if rawID == "" {
		errorx.RespondError(c, fmt.Errorf("%w: token_id", errorsx.ErrRequired))
		return
	}

	tokenID, err := uuid.Parse(rawID)
	if err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: token_id", errorsx.ErrInvalidArgument))
		return
	}

	if err := h.app.RevokeInviteToken(c.Request.Context(), userID, tokenID, role); err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: %s", errorsx.ErrInvalidArgument, err.Error()))
		return
	}

	err := h.app.Register(c.Request.Context(), req.Email, req.Password, req.InviteToken)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.Status(http.StatusCreated)
}

func (h *Handler) VerifyEmail(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		errorx.RespondError(c, fmt.Errorf("%w: token", errorsx.ErrRequired))
		return
	}

	if err := h.app.VerifyEmailByToken(c.Request.Context(), token); err != nil {
		errorx.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: %s", errorsx.ErrInvalidArgument, err.Error()))
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

	resp, err := h.app.Login(c.Request.Context(), input)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, loginResponse{AccessToken: resp.AccessToken, RefreshToken: resp.RefreshToken})
}

func (h *Handler) Logout(c *gin.Context) {
	sessionID, ok := middleware.GetSessionID(c)
	if !ok {
		errorx.RespondError(c, errors.Join(errorsx.ErrInternal, errors.New("gin context: wrong session id")))
		return
	}
	err := h.app.Logout(c.Request.Context(), sessionID)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) LogoutAll(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		errorx.RespondError(c, errors.Join(errorsx.ErrInternal, errors.New("gin cotext: wrong user id")))
		return
	}

	sessionID, ok := middleware.GetSessionID(c)
	if !ok {
		errorx.RespondError(c, errors.Join(errorsx.ErrInternal, errors.New("gin context: wrong session id")))
		return
	}

	err := h.app.LogoutAll(c.Request.Context(), userID, sessionID)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) RefreshToken(c *gin.Context) {
	var req refreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: %s", errorsx.ErrInvalidArgument, err.Error()))
		return
	}

	token := req.RefreshToken

	resp, err := h.app.RefreshToken(c.Request.Context(), token)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, refreshTokenResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	})
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorx.RespondError(c, fmt.Errorf("%w: %s", errorsx.ErrInvalidArgument, err.Error()))
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		errorx.RespondError(c, errors.Join(errorsx.ErrInternal, errors.New("gin cotext: wrong user id")))
		return
	}
	sessionID, ok := middleware.GetSessionID(c)
	if !ok {
		errorx.RespondError(c, errors.Join(errorsx.ErrInternal, errors.New("gin context: wrong session id")))
		return
	}

	err := h.app.ChangePassword(c.Request.Context(), auth.ChangePasswordInput{
		UserID:           userID,
		CurrentSessionID: sessionID,
		CurrentPassword:  req.CurrentPassword,
		NewPassword:      req.NewPassword,
	})
	if err != nil {
		errorx.RespondError(c, err)
		return
	}
	c.Status(http.StatusOK)
}

func (h *Handler) RequestPasswordReset(c *gin.Context) {
	var req passwordResetEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorx.RespondError(c, err)
		return
	}

	err := h.app.RequestPasswordReset(c.Request.Context(), req.Email)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorx.RespondError(c, err)
		return
	}

	err := h.app.ResetPassword(c.Request.Context(), req.ResetPasswordToken, req.NewPassword)
	if err != nil {
		errorx.RespondError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

