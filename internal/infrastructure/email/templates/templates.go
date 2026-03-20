package templates

import (
	"fmt"

	"github.com/pulsoats/main/internal/domain/mailer"
)

const (
	verificationEmailSubject = "[%s] Registration"
	verificationEmailBody    = `Hi,

Please confirm your email to complete your registration:

%s

If you didn’t create an account, you can safely ignore this email.`

	resetPasswordEmailSubject = "[%s] Password reset"
	resetPasswordEmailBody    = `Hi,

We received a request to reset your password.

You can set a new password using the link below:

%s

If you didn’t request a password reset, you can safely ignore this email.`
)

// Verification builds a registration confirmation message.
func Verification(to, appName, link string) mailer.Message {
	return mailer.Message{
		To:      to,
		Subject: fmt.Sprintf(verificationEmailSubject, appName),
		Text:    fmt.Sprintf(verificationEmailBody, link),
	}
}

// PasswordReset builds a password reset message.
func PasswordReset(to, appName, link string) mailer.Message {
	return mailer.Message{
		To:      to,
		Subject: fmt.Sprintf(resetPasswordEmailSubject, appName),
		Text:    fmt.Sprintf(resetPasswordEmailBody, link),
	}
}
