package mailer

import "context"

// Message is a single outgoing email.
type Message struct {
	To      []string
	Subject string
	HTML    string // primary body
	Text    string // plain-text fallback (optional)
}

// Mailer is the domain contract for sending email.
// Implementations live in internal/infrastructure/mailer.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}
