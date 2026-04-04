package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Config holds SMTP configuration for an organization.
type Config struct {
	Host      string `json:"smtp_host"`
	Port      int    `json:"smtp_port"`
	Username  string `json:"smtp_user"`
	Password  string `json:"smtp_pass"`
	FromEmail string `json:"email_from_address"`
	FromName  string `json:"email_from_name"`
	TLS       bool   `json:"smtp_tls"` // true = implicit TLS (port 465), false = STARTTLS (port 587/25)
}

// Message represents an email to send.
type Message struct {
	To       []string
	CC       []string
	Subject  string
	HTMLBody string
	TextBody string // plain-text fallback
}

// Mailer sends emails via SMTP.
type Mailer struct {
	cfg Config
}

// New creates a new Mailer from the given Config.
// Returns nil if the config is not usable (no host).
func New(cfg Config) *Mailer {
	if cfg.Host == "" {
		return nil
	}
	if cfg.Port == 0 {
		if cfg.TLS {
			cfg.Port = 465
		} else {
			cfg.Port = 587
		}
	}
	return &Mailer{cfg: cfg}
}

// addr returns the host:port string for the SMTP server.
func (m *Mailer) addr() string {
	return fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
}

// from returns the formatted "From" header value.
func (m *Mailer) from() string {
	if m.cfg.FromName != "" {
		return fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", m.cfg.FromName), m.cfg.FromEmail)
	}
	return m.cfg.FromEmail
}

// Send sends an email message via SMTP.
func (m *Mailer) Send(ctx context.Context, msg *Message) error {
	if len(msg.To) == 0 {
		return fmt.Errorf("email: no recipients")
	}

	// Build the raw MIME message
	body := m.buildMessage(msg)

	// Collect all recipients
	recipients := make([]string, 0, len(msg.To)+len(msg.CC))
	recipients = append(recipients, msg.To...)
	recipients = append(recipients, msg.CC...)

	// Send via SMTP
	if m.cfg.TLS {
		return m.sendTLS(ctx, recipients, body)
	}
	return m.sendSTARTTLS(ctx, recipients, body)
}

// TestConnection verifies the SMTP credentials by connecting and authenticating.
func (m *Mailer) TestConnection(ctx context.Context) error {
	done := make(chan error, 1)
	go func() {
		if m.cfg.TLS {
			done <- m.testTLS()
		} else {
			done <- m.testSTARTTLS()
		}
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// buildMessage constructs a multipart MIME email.
func (m *Mailer) buildMessage(msg *Message) []byte {
	var b strings.Builder

	boundary := fmt.Sprintf("==WHATOMATE_%d==", time.Now().UnixNano())

	// Headers
	msgID := fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), "whatomate", m.cfg.Host)
	b.WriteString(fmt.Sprintf("Message-ID: %s\r\n", msgID))
	b.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	b.WriteString(fmt.Sprintf("From: %s\r\n", m.from()))
	b.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ", ")))
	if len(msg.CC) > 0 {
		b.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(msg.CC, ", ")))
	}
	b.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject)))
	b.WriteString("MIME-Version: 1.0\r\n")

	encodeQP := func(s string) string {
		var buf bytes.Buffer
		w := quotedprintable.NewWriter(&buf)
		w.Write([]byte(s))
		w.Close()
		return buf.String()
	}

	if msg.HTMLBody != "" && msg.TextBody != "" {
		// Multipart alternative
		b.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
		b.WriteString("\r\n")

		// Plain text part
		b.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		b.WriteString("\r\n")
		b.WriteString(encodeQP(msg.TextBody))
		b.WriteString("\r\n")

		// HTML part
		b.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		b.WriteString("\r\n")
		b.WriteString(encodeQP(msg.HTMLBody))
		b.WriteString("\r\n")

		b.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else if msg.HTMLBody != "" {
		b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		b.WriteString("\r\n")
		b.WriteString(encodeQP(msg.HTMLBody))
	} else {
		b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		b.WriteString("\r\n")
		b.WriteString(encodeQP(msg.TextBody))
	}

	return []byte(b.String())
}

// sendSTARTTLS sends email using STARTTLS (ports 587/25).
func (m *Mailer) sendSTARTTLS(_ context.Context, recipients []string, body []byte) error {
	conn, err := net.DialTimeout("tcp", m.addr(), 10*time.Second)
	if err != nil {
		return fmt.Errorf("email: dial failed: %w", err)
	}

	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("email: smtp client failed: %w", err)
	}
	defer func() { _ = client.Quit() }()

	// Upgrade to TLS
	tlsCfg := &tls.Config{ServerName: m.cfg.Host}
	if err := client.StartTLS(tlsCfg); err != nil {
		return fmt.Errorf("email: STARTTLS failed: %w", err)
	}

	return m.sendViaClient(client, recipients, body)
}

// sendTLS sends email using implicit TLS (port 465).
func (m *Mailer) sendTLS(_ context.Context, recipients []string, body []byte) error {
	tlsCfg := &tls.Config{ServerName: m.cfg.Host}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", m.addr(), tlsCfg)
	if err != nil {
		return fmt.Errorf("email: TLS dial failed: %w", err)
	}

	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("email: smtp client failed: %w", err)
	}
	defer func() { _ = client.Quit() }()

	return m.sendViaClient(client, recipients, body)
}

// sendViaClient handles auth + envelope + data via an smtp.Client.
func (m *Mailer) sendViaClient(client *smtp.Client, recipients []string, body []byte) error {
	// Authenticate
	if m.cfg.Username != "" {
		auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("email: auth failed: %w", err)
		}
	}

	// Set sender
	if err := client.Mail(m.cfg.FromEmail); err != nil {
		return fmt.Errorf("email: MAIL FROM failed: %w", err)
	}

	// Set recipients
	for _, rcpt := range recipients {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("email: RCPT TO <%s> failed: %w", rcpt, err)
		}
	}

	// Write message body
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("email: DATA failed: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("email: write body failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("email: close data failed: %w", err)
	}

	return nil
}

// testSTARTTLS verifies STARTTLS connection + auth.
func (m *Mailer) testSTARTTLS() error {
	conn, err := net.DialTimeout("tcp", m.addr(), 10*time.Second)
	if err != nil {
		return fmt.Errorf("email: connection refused on %s: %w", m.addr(), err)
	}

	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("email: failed to create SMTP client: %w", err)
	}
	defer func() { _ = client.Quit() }()

	tlsCfg := &tls.Config{ServerName: m.cfg.Host}
	if err := client.StartTLS(tlsCfg); err != nil {
		return fmt.Errorf("email: STARTTLS upgrade failed: %w", err)
	}

	if m.cfg.Username != "" {
		auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("email: authentication failed (check username/password): %w", err)
		}
	}

	return nil
}

// testTLS verifies implicit TLS connection + auth.
func (m *Mailer) testTLS() error {
	tlsCfg := &tls.Config{ServerName: m.cfg.Host}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", m.addr(), tlsCfg)
	if err != nil {
		return fmt.Errorf("email: TLS connection to %s failed: %w", m.addr(), err)
	}

	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("email: failed to create SMTP client: %w", err)
	}
	defer func() { _ = client.Quit() }()

	if m.cfg.Username != "" {
		auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("email: authentication failed (check username/password): %w", err)
		}
	}

	return nil
}
