package auth

type registerRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	InviteToken string `json:"invite_token" binding:"required"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required, email"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type createInviteTokenResponse struct {
	InviteToken string `json:"invite_token"`
	InviteLink  string `json:"invite_link"`
}

type refreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type refreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type passwordResetEmailRequest struct {
	Email string `json:"email" binding:"required"`
}

type resetPasswordRequest struct {
	ResetPasswordToken string `json:"reset_password_token" binding:"required"`
	NewPassword        string `json:"new_password" binding:"required"`
}
