package mail

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Security — как защищается соединение с почтовым сервером.
type Security string

const (
	// SecurityStartTLS — обычное подключение с последующим STARTTLS.
	// Так работает большинство релеев на порту 587.
	SecurityStartTLS Security = "starttls"
	// SecurityTLS — TLS с первого байта, порт 465.
	SecurityTLS Security = "tls"
	// SecurityNone — без шифрования. Допустимо только для релея на самой
	// машине; пароль по такому каналу не отправляется.
	SecurityNone Security = "none"
)

func (s Security) Valid() bool {
	switch s {
	case SecurityStartTLS, SecurityTLS, SecurityNone:
		return true
	}
	return false
}

// SMTPConfig — настройки почтового сервера.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	// From — адрес отправителя, допускается вид «Имя <box@example.com>».
	From     string
	Security Security
	Timeout  time.Duration
}

// SMTPMailer отправляет письма через внешний почтовый сервер.
type SMTPMailer struct {
	cfg SMTPConfig
}

// NewSMTPMailer проверяет настройки и создаёт отправщика.
func NewSMTPMailer(cfg SMTPConfig) (*SMTPMailer, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("не указан адрес почтового сервера")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, fmt.Errorf("некорректный порт почтового сервера: %d", cfg.Port)
	}
	if _, err := mail.ParseAddress(cfg.From); err != nil {
		return nil, fmt.Errorf("некорректный адрес отправителя %q: %w", cfg.From, err)
	}
	if !cfg.Security.Valid() {
		return nil, fmt.Errorf("неизвестный режим защиты соединения: %q", cfg.Security)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	return &SMTPMailer{cfg: cfg}, nil
}

func (m *SMTPMailer) SendLoginLink(ctx context.Context, email, link string) error {
	return m.send(ctx, loginMessage(m.cfg.From, email, link))
}

func (m *SMTPMailer) SendInvite(ctx context.Context, email, link, leaderName string) error {
	if strings.TrimSpace(email) == "" {
		// Приглашение без почты — обычное дело: ссылку передают вручную.
		return nil
	}
	return m.send(ctx, inviteMessage(m.cfg.From, email, link, leaderName))
}

func (m *SMTPMailer) send(ctx context.Context, msg message) error {
	msg.Date = time.Now()
	id, err := messageID(m.cfg.From)
	if err != nil {
		return err
	}
	msg.MessageID = id

	body, err := msg.build()
	if err != nil {
		return err
	}

	from, err := mail.ParseAddress(m.cfg.From)
	if err != nil {
		return fmt.Errorf("некорректный адрес отправителя: %w", err)
	}
	to, err := mail.ParseAddress(msg.To)
	if err != nil {
		return fmt.Errorf("некорректный адрес получателя: %w", err)
	}

	client, err := m.connect(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := m.authenticate(client); err != nil {
		return err
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("передача письма: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("завершение передачи: %w", err)
	}
	return client.Quit()
}

func (m *SMTPMailer) addr() string {
	return net.JoinHostPort(m.cfg.Host, strconv.Itoa(m.cfg.Port))
}

func (m *SMTPMailer) connect(ctx context.Context) (*smtp.Client, error) {
	dialer := &net.Dialer{Timeout: m.cfg.Timeout}

	if m.cfg.Security == SecurityTLS {
		conn, err := tls.DialWithDialer(dialer, "tcp", m.addr(), &tls.Config{ServerName: m.cfg.Host})
		if err != nil {
			return nil, fmt.Errorf("подключение к почтовому серверу: %w", err)
		}
		client, err := smtp.NewClient(conn, m.cfg.Host)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("рукопожатие с почтовым сервером: %w", err)
		}
		return client, nil
	}

	conn, err := dialer.DialContext(ctx, "tcp", m.addr())
	if err != nil {
		return nil, fmt.Errorf("подключение к почтовому серверу: %w", err)
	}
	// Крайний срок на всю сессию: без него зависший сервер держал бы
	// обработчик входа до бесконечности.
	_ = conn.SetDeadline(time.Now().Add(m.cfg.Timeout))

	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("рукопожатие с почтовым сервером: %w", err)
	}

	if m.cfg.Security == SecurityStartTLS {
		if err := client.StartTLS(&tls.Config{ServerName: m.cfg.Host}); err != nil {
			client.Close()
			return nil, fmt.Errorf("STARTTLS: %w", err)
		}
	}
	return client, nil
}

// authenticate входит на сервер, если задан логин.
//
// По незашифрованному каналу пароль не отправляется: net/smtp на это и сам
// не согласится, но лучше отказать с внятным сообщением, чем получить
// «unencrypted connection» из недр библиотеки.
func (m *SMTPMailer) authenticate(client *smtp.Client) error {
	if m.cfg.Username == "" {
		return nil
	}
	if m.cfg.Security == SecurityNone {
		return fmt.Errorf("вход на почтовый сервер запрошен, но соединение не шифруется")
	}
	if ok, _ := client.Extension("AUTH"); !ok {
		return fmt.Errorf("почтовый сервер не предлагает аутентификацию")
	}
	auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("вход на почтовый сервер: %w", err)
	}
	return nil
}

// messageID собирает идентификатор письма из случайной части и домена
// отправителя. Без него почтовые сервисы охотнее считают письмо спамом.
func messageID(from string) (string, error) {
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return "", fmt.Errorf("некорректный адрес отправителя: %w", err)
	}
	domain := addr.Address
	if at := strings.LastIndex(domain, "@"); at >= 0 {
		domain = domain[at+1:]
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("генерация идентификатора письма: %w", err)
	}
	return hex.EncodeToString(buf) + "@" + domain, nil
}
