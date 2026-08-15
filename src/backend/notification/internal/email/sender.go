package email

import "context"

// Message is an auth-related email payload (verification, password reset).
type Message struct {
	To      string
	Subject string
	Body    string
}

// Sender delivers transactional auth emails.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}
