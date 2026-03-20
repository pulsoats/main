package mailer

import "context"

// Message describes contents passed to a Sender implementation.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// Sender abstracts email delivery transport.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}
