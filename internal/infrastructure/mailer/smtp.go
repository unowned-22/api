package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	domainmailer "github.com/unowned-22/api/internal/domain/mailer"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type SMTPMailer struct {
	cfg Config
}

func New(cfg Config) *SMTPMailer {
	return &SMTPMailer{cfg: cfg}
}

func (m *SMTPMailer) Send(ctx context.Context, msg domainmailer.Message) error {
	if len(msg.To) == 0 {
		return errors.New("email recipient list cannot be empty")
	}

	dialer := &net.Dialer{}
	dialer.Timeout = 10 * time.Second

	conn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port))
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Quit()

	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName: m.cfg.Host,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if m.cfg.Username != "" || m.cfg.Password != "" {
		auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	from := m.cfg.From
	if from == "" {
		from = "No Reply"
	}

	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = strings.Join(msg.To, ", ")
	headers["Subject"] = msg.Subject
	headers["MIME-Version"] = "1.0"

	body, err := buildMessageBody(msg)
	if err != nil {
		return err
	}

	msgBytes := new(bytes.Buffer)
	for k, v := range headers {
		fmt.Fprintf(msgBytes, "%s: %s\r\n", k, v)
	}
	fmt.Fprintf(msgBytes, "\r\n%s", body)

	if err := client.Mail(extractAddress(from)); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	for _, recipient := range msg.To {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("smtp rcpt to %s: %w", recipient, err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	defer writer.Close()

	if _, err := writer.Write(msgBytes.Bytes()); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}

	return nil
}

func buildMessageBody(msg domainmailer.Message) (string, error) {
	hasHTML := strings.TrimSpace(msg.HTML) != ""
	hasText := strings.TrimSpace(msg.Text) != ""

	if hasHTML && hasText {
		boundary := "==BOUNDARY=="
		var body bytes.Buffer
		body.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")
		body.WriteString("--" + boundary + "\r\n")
		body.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		body.WriteString(msg.Text + "\r\n\r\n")
		body.WriteString("--" + boundary + "\r\n")
		body.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		body.WriteString(msg.HTML + "\r\n\r\n")
		body.WriteString("--" + boundary + "--\r\n")
		return body.String(), nil
	}

	if hasHTML {
		return "Content-Type: text/html; charset=UTF-8\r\n\r\n" + msg.HTML, nil
	}
	if hasText {
		return "Content-Type: text/plain; charset=UTF-8\r\n\r\n" + msg.Text, nil
	}

	return "", errors.New("email must contain either HTML or Text body")
}

func extractAddress(from string) string {
	if strings.Contains(from, "<") && strings.Contains(from, ">") {
		start := strings.Index(from, "<")
		end := strings.Index(from, ">")
		if start < end {
			return strings.TrimSpace(from[start+1 : end])
		}
	}
	return strings.TrimSpace(from)
}
