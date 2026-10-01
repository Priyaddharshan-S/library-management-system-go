package notification

// Sender is the abstraction the rest of the app depends on. Business logic
// (in the service layer) calls Send() without knowing or caring whether
// the email actually goes out over SMTP or just gets printed to a
// terminal — that decision is made once, in main.go, based on config.
type Sender interface {
	Send(to, subject, body string) error
}
