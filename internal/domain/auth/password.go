package auth

type ChangePasswordInput struct {
	UserID           int64
	CurrentSessionID int64
	CurrentPassword  string
	NewPassword      string
}
