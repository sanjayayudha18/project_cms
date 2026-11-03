package notification

import (
	"bufio"
	"context"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"
)

// fakeSMTPServer accepts one connection, speaks just enough SMTP for
// net/smtp (no STARTTLS, no AUTH) and returns the DATA payload.
func fakeSMTPServer(t *testing.T) (addr string, data <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	out := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		write := func(s string) { _, _ = io.WriteString(conn, s+"\r\n") }
		write("220 fake ESMTP")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				write("250 fake")
			case strings.HasPrefix(cmd, "MAIL"), strings.HasPrefix(cmd, "RCPT"):
				write("250 OK")
			case cmd == "DATA":
				write("354 go ahead")
				var b strings.Builder
				for {
					l, err := r.ReadString('\n')
					if err != nil {
						return
					}
					if l == ".\r\n" {
						break
					}
					b.WriteString(l)
				}
				out <- b.String()
				write("250 queued")
			case cmd == "QUIT":
				write("221 bye")
				return
			default:
				write("250 OK")
			}
		}
	}()
	return ln.Addr().String(), out
}

func TestSMTPMailer_SendsUTF8PlainText(t *testing.T) {
	addr, data := fakeSMTPServer(t)
	host, port, _ := net.SplitHostPort(addr)
	m := &SMTPMailer{Host: host, Port: port, From: "cms-noreply@bank.co.id", DialTimeout: time.Second, IOTimeout: 2 * time.Second}

	subject := "Kelebihan kuota kunjungan — ATM 001"
	body := "Vendor request REP001: ATM T001 melebihi kuota.\n.baris diawali titik"
	if err := m.Send(context.Background(), "spv@bank.co.id", subject, body); err != nil {
		t.Fatal(err)
	}

	raw := <-data
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("parse message: %v\n%s", err, raw)
	}
	gotSubject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil || gotSubject != subject {
		t.Fatalf("subject = %q (%v), want %q", gotSubject, err, subject)
	}
	if !strings.HasPrefix(msg.Header.Get("Content-Type"), "text/plain; charset=UTF-8") {
		t.Fatalf("content-type = %q", msg.Header.Get("Content-Type"))
	}
	if msg.Header.Get("To") != "spv@bank.co.id" || msg.Header.Get("From") != "cms-noreply@bank.co.id" {
		t.Fatalf("to/from = %q/%q", msg.Header.Get("To"), msg.Header.Get("From"))
	}
	decoded, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if err != nil {
		t.Fatal(err)
	}
	// Dot-stuffing is undone by the server side of the protocol; the fake
	// server keeps it, so strip it the same way a real MTA would. DATA always
	// ends with a line break (textproto.DotWriter), hence the TrimSuffix.
	got := strings.ReplaceAll(strings.ReplaceAll(string(decoded), "\r\n", "\n"), "\n..", "\n.")
	got = strings.TrimSuffix(got, "\n")
	if got != body {
		t.Fatalf("body = %q, want %q", got, body)
	}
}

func TestSMTPMailer_RejectsHeaderInjection(t *testing.T) {
	m := &SMTPMailer{Host: "127.0.0.1", Port: "1", From: "a@b.co"}
	if err := m.Send(context.Background(), "x@b.co\r\nBcc: evil@b.co", "s", "b"); err == nil {
		t.Fatal("expected an error for CR/LF in recipient")
	}
}

func TestSMTPMailer_TimesOutOnSilentServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			defer conn.Close()
			time.Sleep(3 * time.Second) // never sends the 220 greeting
		}
	}()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	m := &SMTPMailer{Host: host, Port: port, From: "a@b.co", DialTimeout: time.Second, IOTimeout: 200 * time.Millisecond}

	start := time.Now()
	if err := m.Send(context.Background(), "x@b.co", "s", "b"); err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Send hung for %v", elapsed)
	}
}

func TestSMTPMailer_DialFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close() // nothing listens any more
	host, port, _ := net.SplitHostPort(addr)
	m := &SMTPMailer{Host: host, Port: port, From: "a@b.co", DialTimeout: time.Second}
	if err := m.Send(context.Background(), "x@b.co", "s", "b"); err == nil || !strings.Contains(err.Error(), "smtp dial") {
		t.Fatalf("err = %v, want smtp dial error", err)
	}
}

func TestSMTPMailer_DefaultTimeouts(t *testing.T) {
	if orDefault(0, time.Second) != time.Second || orDefault(2*time.Second, time.Second) != 2*time.Second {
		t.Fatal("orDefault")
	}
}
