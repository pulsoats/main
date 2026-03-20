package auth

type EmailType int

const (
	EmailTypeVerification EmailType = iota
	EmailTypePasswordReset
)
