package mailer

import "context"

// Message описывает содержимое сообщения, передаваемое в Sender.Send.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// Sender является абстракцией сервиса отправки email.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}
