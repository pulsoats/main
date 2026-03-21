package templates

import (
	"fmt"

	"github.com/pulsoats/main/internal/domain/mailer"
)

const (
	verificationEmailSubject = "[%s] Подтверждение регистрации"
	verificationEmailBody    = `Здравствуйте!

Подтвердите адрес электронной почты, чтобы завершить регистрацию:

%s

Если вы не создавали аккаунт, просто проигнорируйте это письмо.`

	resetPasswordEmailSubject = "[%s] Сброс пароля"
	resetPasswordEmailBody    = `Здравствуйте!

Мы получили запрос на сброс вашего пароля.

Перейдите по ссылке ниже, чтобы установить новый пароль:

%s

Если вы не запрашивали сброс, просто проигнорируйте это письмо.`
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
