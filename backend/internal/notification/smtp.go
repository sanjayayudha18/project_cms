package notification

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

const (
	defaultDialTimeout = 10 * time.Second
	defaultIOTimeout   = 30 * time.Second
)

// SMTPMailer sends plain-text UTF-8 email through the company SMTP relay
// using only net/smtp (no new dependency, plan D2/D3). net/smtp.SendMail has
// no timeouts, so the connection is dialled here with a dial timeout and an
// overall deadline; STARTTLS is used whenever the server offers it, AUTH
// PLAIN only when User is set.
type SMTPMailer struct {
	Host, Port, User, Password, From string
	DialTimeout, IOTimeout           time.Duration
}

func (m *SMTPMailer) Send(ctx context.Context, to, subject, body string) error {
	if strings.ContainsAny(to, "\r\n") {
		return errors.New("smtp: invalid recipient")
	}
	if _, err := mail.ParseAddress(to); err != nil {
		return fmt.Errorf("smtp: invalid recipient: %w", err)
	}
	msg, err := m.buildMessage(to, subject, body)
	if err != nil {
		return err
	}

	dialer := net.Dialer{Timeout: orDefault(m.DialTimeout, defaultDialTimeout)}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(m.Host, m.Port))
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	if err := conn.SetDeadline(time.Now().Add(orDefault(m.IOTimeout, defaultIOTimeout))); err != nil {
		conn.Close()
		return fmt.Errorf("smtp deadline: %w", err)
	}
	c, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp greeting: %w", err)
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: m.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	if m.User != "" {
		if err := c.Auth(smtp.PlainAuth("", m.User, m.Password, m.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(m.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp data close: %w", err)
	}
	return c.Quit()
}

func (m *SMTPMailer) buildMessage(to, subject, body string) ([]byte, error) {
	subject = strings.NewReplacer("\r", "", "\n", " ").Replace(subject)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", m.From)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")

	qp := quotedprintable.NewWriter(&buf)
	if _, err := qp.Write([]byte(strings.ReplaceAll(body, "\n", "\r\n"))); err != nil {
		return nil, fmt.Errorf("encode body: %w", err)
	}
	if err := qp.Close(); err != nil {
		return nil, fmt.Errorf("encode body: %w", err)
	}
	return buf.Bytes(), nil
}

func orDefault(d, def time.Duration) time.Duration {
	if d <= 0 {
		return def
	}
	return d
}
