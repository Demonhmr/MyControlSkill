package mail

import (
	"bufio"
	"context"
	"encoding/base64"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTP — минимальный SMTP-сервер на время теста.
//
// Настоящий почтовый сервер в тестах недоступен, а проверять хочется именно
// диалог: без него легко отправить синтаксически верное письмо, которое ни
// один сервер не примет.
type fakeSMTP struct {
	addr string

	mu       sync.Mutex
	sessions []session
	// advertiseAuth — предлагать ли AUTH в ответе на EHLO.
	advertiseAuth bool
	// rejectRcpt — отвечать отказом на RCPT TO.
	rejectRcpt bool
}

type session struct {
	From    string
	To      string
	Data    string
	AuthRaw string
}

func startFakeSMTP(t *testing.T, configure func(*fakeSMTP)) *fakeSMTP {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("не удалось занять порт: %v", err)
	}
	s := &fakeSMTP{addr: listener.Addr().String()}
	if configure != nil {
		configure(s)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go s.handle(conn)
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		<-done
	})
	return s
}

func (s *fakeSMTP) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	reply := func(line string) {
		w.WriteString(line + "\r\n")
		w.Flush()
	}

	reply("220 fake ESMTP")

	var current session
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimRight(line, "\r\n")
		upper := strings.ToUpper(cmd)

		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			if s.advertiseAuth {
				reply("250-fake")
				reply("250 AUTH PLAIN")
			} else {
				reply("250 fake")
			}
		case strings.HasPrefix(upper, "AUTH PLAIN"):
			current.AuthRaw = strings.TrimSpace(cmd[len("AUTH PLAIN"):])
			reply("235 ok")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			current.From = strings.TrimSpace(cmd[len("MAIL FROM:"):])
			reply("250 ok")
		case strings.HasPrefix(upper, "RCPT TO:"):
			if s.rejectRcpt {
				reply("550 нет такого ящика")
				continue
			}
			current.To = strings.TrimSpace(cmd[len("RCPT TO:"):])
			reply("250 ok")
		case upper == "DATA":
			reply("354 давай")
			var body strings.Builder
			for {
				dataLine, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if dataLine == ".\r\n" {
					break
				}
				body.WriteString(dataLine)
			}
			current.Data = body.String()
			s.mu.Lock()
			s.sessions = append(s.sessions, current)
			s.mu.Unlock()
			current = session{}
			reply("250 принято")
		case upper == "QUIT":
			reply("221 пока")
			return
		case upper == "RSET":
			current = session{}
			reply("250 ok")
		default:
			reply("250 ok")
		}
	}
}

func (s *fakeSMTP) received(t *testing.T) session {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sessions) == 0 {
		t.Fatal("сервер не получил ни одного письма")
	}
	return s.sessions[len(s.sessions)-1]
}

func newMailer(t *testing.T, addr string, adjust func(*SMTPConfig)) *SMTPMailer {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("разбор адреса: %v", err)
	}
	port := 0
	for _, c := range portStr {
		port = port*10 + int(c-'0')
	}

	cfg := SMTPConfig{
		Host:     host,
		Port:     port,
		From:     "Компас руководителя <no-reply@example.com>",
		Security: SecurityNone,
		Timeout:  5 * time.Second,
	}
	if adjust != nil {
		adjust(&cfg)
	}
	m, err := NewSMTPMailer(cfg)
	if err != nil {
		t.Fatalf("NewSMTPMailer: %v", err)
	}
	return m
}

// decodeBody достаёт текст письма из base64-тела.
func decodeBody(t *testing.T, raw string) string {
	t.Helper()
	parts := strings.SplitN(raw, "\r\n\r\n", 2)
	if len(parts) != 2 {
		t.Fatalf("в письме нет разделителя заголовков и тела:\n%s", raw)
	}
	encoded := strings.ReplaceAll(parts[1], "\r\n", "")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("тело не разбирается как base64: %v", err)
	}
	return string(decoded)
}

func TestОтправкаСсылкиДляВхода(t *testing.T) {
	server := startFakeSMTP(t, nil)
	mailer := newMailer(t, server.addr, nil)

	link := "https://compass.example.com/api/auth/callback?token=abc123"
	if err := mailer.SendLoginLink(context.Background(), "lead@example.com", link); err != nil {
		t.Fatalf("SendLoginLink: %v", err)
	}

	got := server.received(t)
	if !strings.Contains(got.From, "no-reply@example.com") {
		t.Errorf("MAIL FROM = %q", got.From)
	}
	if !strings.Contains(got.To, "lead@example.com") {
		t.Errorf("RCPT TO = %q", got.To)
	}

	// Кириллица в теме обязана быть закодирована, иначе доедет как «????».
	if !strings.Contains(got.Data, "Subject: =?utf-8?q?") {
		t.Errorf("тема не закодирована по RFC 2047:\n%s", got.Data)
	}
	for _, header := range []string{"MIME-Version: 1.0", `charset="utf-8"`, "Content-Transfer-Encoding: base64", "Message-ID: <", "Auto-Submitted: auto-generated"} {
		if !strings.Contains(got.Data, header) {
			t.Errorf("в письме нет заголовка %q", header)
		}
	}

	body := decodeBody(t, got.Data)
	if !strings.Contains(body, link) {
		t.Errorf("в теле нет ссылки:\n%s", body)
	}
	if !strings.Contains(body, "15 минут") {
		t.Errorf("в теле нет срока жизни ссылки:\n%s", body)
	}
}

func TestОтправкаПриглашения(t *testing.T) {
	server := startFakeSMTP(t, nil)
	mailer := newMailer(t, server.addr, nil)

	link := "https://compass.example.com/s/ТОКЕН"
	if err := mailer.SendInvite(context.Background(), "peer@example.com", link, "Пётр Иванов"); err != nil {
		t.Fatalf("SendInvite: %v", err)
	}

	body := decodeBody(t, server.received(t).Data)
	if !strings.Contains(body, "Пётр Иванов") {
		t.Errorf("в теле нет имени оцениваемого:\n%s", body)
	}
	if !strings.Contains(body, link) {
		t.Errorf("в теле нет ссылки:\n%s", body)
	}
	// Респондент должен понимать, что его ответы обезличены, — иначе он
	// отвечает осторожнее, чем думает.
	if !strings.Contains(body, "обезличены") {
		t.Errorf("в теле нет обещания анонимности:\n%s", body)
	}
}

func TestПриглашениеБезИмениНеОставляетДырку(t *testing.T) {
	server := startFakeSMTP(t, nil)
	mailer := newMailer(t, server.addr, nil)

	if err := mailer.SendInvite(context.Background(), "peer@example.com", "https://x/s/t", "  "); err != nil {
		t.Fatalf("SendInvite: %v", err)
	}

	body := decodeBody(t, server.received(t).Data)
	if !strings.Contains(body, "вашего руководителя") {
		t.Errorf("пустое имя не заменено формулировкой:\n%s", body)
	}
}

func TestПриглашениеБезПочтыНеОтправляется(t *testing.T) {
	server := startFakeSMTP(t, nil)
	mailer := newMailer(t, server.addr, nil)

	// Ссылку можно передать и вне почты — это не ошибка.
	if err := mailer.SendInvite(context.Background(), "", "https://x/s/t", "Пётр"); err != nil {
		t.Fatalf("SendInvite без адреса вернул ошибку: %v", err)
	}

	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.sessions) != 0 {
		t.Errorf("сервер получил письмо без адресата")
	}
}

func TestОтказСервераВозвращаетсяОшибкой(t *testing.T) {
	server := startFakeSMTP(t, func(s *fakeSMTP) { s.rejectRcpt = true })
	mailer := newMailer(t, server.addr, nil)

	err := mailer.SendLoginLink(context.Background(), "lead@example.com", "https://x")
	if err == nil {
		t.Fatal("отказ сервера проглочен")
	}
	if !strings.Contains(err.Error(), "RCPT TO") {
		t.Errorf("ошибка не указывает на шаг диалога: %v", err)
	}
}

func TestВходНаСерверПоОткрытомуКаналуЗапрещён(t *testing.T) {
	server := startFakeSMTP(t, func(s *fakeSMTP) { s.advertiseAuth = true })
	mailer := newMailer(t, server.addr, func(c *SMTPConfig) {
		c.Username = "user"
		c.Password = "секрет"
		c.Security = SecurityNone
	})

	err := mailer.SendLoginLink(context.Background(), "lead@example.com", "https://x")
	if err == nil {
		t.Fatal("пароль ушёл бы по незашифрованному каналу")
	}
	if !strings.Contains(err.Error(), "не шифруется") {
		t.Errorf("непонятная причина отказа: %v", err)
	}

	server.mu.Lock()
	defer server.mu.Unlock()
	if len(server.sessions) != 0 {
		t.Error("письмо всё-таки ушло")
	}
}

func TestНастройкиПроверяются(t *testing.T) {
	base := SMTPConfig{Host: "smtp.example.com", Port: 587, From: "a@example.com", Security: SecurityStartTLS}

	cases := map[string]func(*SMTPConfig){
		"нет адреса сервера": func(c *SMTPConfig) { c.Host = "" },
		"порт вне диапазона": func(c *SMTPConfig) { c.Port = 70000 },
		"нулевой порт":       func(c *SMTPConfig) { c.Port = 0 },
		"кривой отправитель": func(c *SMTPConfig) { c.From = "не адрес" },
		"неизвестный режим":  func(c *SMTPConfig) { c.Security = "какой-то" },
	}
	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := base
			spoil(&cfg)
			if _, err := NewSMTPMailer(cfg); err == nil {
				t.Error("некорректные настройки приняты")
			}
		})
	}

	if _, err := NewSMTPMailer(base); err != nil {
		t.Errorf("корректные настройки отвергнуты: %v", err)
	}
}

func TestПодстановкаЗаголовковНеПроходит(t *testing.T) {
	// Адрес с переводом строки — классический способ дописать в письмо
	// чужого получателя. Разбор адреса такое отвергает, но пусть это
	// зафиксирует тест.
	msg := message{
		From:    "no-reply@example.com",
		To:      "victim@example.com\r\nBcc: attacker@example.com",
		Subject: "Тема",
		Body:    "текст",
		Date:    time.Now(),
	}
	if _, err := msg.build(); err == nil {
		t.Error("адрес с переводом строки принят")
	}

	msg.To = "victim@example.com"
	msg.Subject = "Тема\r\nBcc: attacker@example.com"
	if _, err := msg.build(); err == nil {
		t.Error("тема с переводом строки принята")
	}
}

func TestДлинныйРусскийТекстНеЛомаетСтроки(t *testing.T) {
	msg := message{
		From:    "no-reply@example.com",
		To:      "lead@example.com",
		Subject: "Тема",
		Body:    strings.Repeat("Длинный русский текст без переводов строки. ", 60),
		Date:    time.Now(),
	}
	raw, err := msg.build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Предел строки в SMTP — 998 байт; кириллица занимает по два байта, и
	// абзац переваливает за него незаметно.
	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 998 {
			t.Fatalf("строка длиной %d байт", len(line))
		}
	}
	if got := decodeBody(t, string(raw)); got != msg.Body {
		t.Error("тело не совпадает с исходным после кодирования")
	}
}
