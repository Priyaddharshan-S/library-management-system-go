package notification

import "log"

// ConsoleSender "sends" email by printing it to the terminal instead of
// actually delivering it. Used automatically when no SMTP credentials are
// configured, so you can test the reservation/notification flow without
// setting up a real email account first.
type ConsoleSender struct{}

func NewConsoleSender() *ConsoleSender {
	return &ConsoleSender{}
}

func (s *ConsoleSender) Send(to, subject, body string) error {
	log.Printf("\n---- EMAIL (console mode — not actually sent) ----\nTo: %s\nSubject: %s\n\n%s\n---------------------------------------------------\n", to, subject, body)
	return nil
}
