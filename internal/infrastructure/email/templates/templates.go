package templates

import (
	"fmt"

	"github.com/pulsoats/main/internal/domain/mailer"
)

const (
	accountExpiredEmailSubject = "[%s] Ключи аккаунта истекли"
	accountExpiredEmailBody    = `Здравствуйте!

Ключи API аккаунта "%s" (биржа: %s) истекли.

Пожалуйста, обновите ключи в панели управления, чтобы восстановить работу.`

	accountExpiryReminderEmailSubject = "[%s] Ключи аккаунта истекают через %d дней"
	accountExpiryReminderEmailBody    = `Здравствуйте!

Ключи API аккаунта "%s" (биржа: %s) истекают через %d дней.

Обновите ключи заранее, чтобы не прерывать работу.`

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

// AccountExpired builds a message notifying that API keys have expired.
func AccountExpired(to, appName, accountName, exchange string) mailer.Message {
	return mailer.Message{
		To:      to,
		Subject: fmt.Sprintf(accountExpiredEmailSubject, appName),
		Text:    fmt.Sprintf(accountExpiredEmailBody, accountName, exchange),
	}
}

// AccountExpiryReminder builds a reminder message that API keys are expiring soon.
func AccountExpiryReminder(to, appName, accountName, exchange string, daysRemaining int) mailer.Message {
	return mailer.Message{
		To:      to,
		Subject: fmt.Sprintf(accountExpiryReminderEmailSubject, appName, daysRemaining),
		Text:    fmt.Sprintf(accountExpiryReminderEmailBody, accountName, exchange, daysRemaining),
	}
}

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
