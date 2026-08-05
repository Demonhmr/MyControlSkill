package mail

import (
	"encoding/base64"
	"fmt"
	"mime"
	"net/mail"
	"strings"
	"time"
)

// message — письмо, готовое к отправке.
type message struct {
	From    string
	To      string
	Subject string
	Body    string
	Date    time.Time
	// MessageID без угловых скобок. Пустой — заголовок не ставится.
	MessageID string
}

// build собирает письмо в виде, пригодном для передачи в DATA.
//
// Тело кодируется в base64 целиком. Причина не в экономии: тексты русские,
// в quoted-printable они превращаются в нечитаемую кашу из =D0=9F, а строки
// SMTP ограничены 998 байтами, и длинный абзац кириллицы этот предел
// перешагивает незаметно.
func (m message) build() ([]byte, error) {
	from, err := mail.ParseAddress(m.From)
	if err != nil {
		return nil, fmt.Errorf("некорректный адрес отправителя %q: %w", m.From, err)
	}
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return nil, fmt.Errorf("некорректный адрес получателя %q: %w", m.To, err)
	}

	// Тема приходит из кода, но проверка дешёвая, а подстановка заголовка
	// через перевод строки — классический способ дописать в письмо чужой
	// получатель.
	if strings.ContainsAny(m.Subject, "\r\n") {
		return nil, fmt.Errorf("тема письма содержит перевод строки")
	}

	var b strings.Builder
	writeHeader(&b, "From", from.String())
	writeHeader(&b, "To", to.String())
	// RFC 2047: кириллица в теме иначе доедет как «????».
	writeHeader(&b, "Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	writeHeader(&b, "Date", m.Date.Format(time.RFC1123Z))
	if m.MessageID != "" {
		writeHeader(&b, "Message-ID", "<"+m.MessageID+">")
	}
	writeHeader(&b, "MIME-Version", "1.0")
	writeHeader(&b, "Content-Type", `text/plain; charset="utf-8"`)
	writeHeader(&b, "Content-Transfer-Encoding", "base64")
	// Письмо служебное: ответы на него читать некому, и в автоответчики
	// оно попадать не должно.
	writeHeader(&b, "Auto-Submitted", "auto-generated")
	b.WriteString("\r\n")

	encoded := base64.StdEncoding.EncodeToString([]byte(m.Body))
	// 76 символов в строке — предел, заданный для base64 в MIME.
	const lineLen = 76
	for i := 0; i < len(encoded); i += lineLen {
		end := min(i+lineLen, len(encoded))
		b.WriteString(encoded[i:end])
		b.WriteString("\r\n")
	}

	return []byte(b.String()), nil
}

func writeHeader(b *strings.Builder, name, value string) {
	b.WriteString(name)
	b.WriteString(": ")
	b.WriteString(value)
	b.WriteString("\r\n")
}

// loginMessage — письмо со ссылкой для входа.
func loginMessage(from, to, link string) message {
	return message{
		From:    from,
		To:      to,
		Subject: "Вход в «Компас руководителя»",
		Body: "Здравствуйте!\n\n" +
			"Чтобы войти, откройте ссылку:\n\n" +
			link + "\n\n" +
			"Ссылка действует 15 минут и сработает один раз.\n" +
			"Если вход запрашивали не вы, просто удалите это письмо — ничего не произойдёт.\n",
	}
}

// inviteMessage — письмо с приглашением заполнить анкету 360°.
func inviteMessage(from, to, link, leaderName string) message {
	who := strings.TrimSpace(leaderName)
	if who == "" {
		who = "вашего руководителя"
	}
	return message{
		From:    from,
		To:      to,
		Subject: "Просьба дать обратную связь 360°",
		Body: "Здравствуйте!\n\n" +
			"Вас просят оценить " + who + " — это займёт около десяти минут.\n\n" +
			link + "\n\n" +
			"Ответы обезличены: кто и как ответил, не видно никому.\n" +
			"Усреднённые оценки увидит оцениваемый, а если он состоит в организации —\n" +
			"то и её HR-служба.\n" +
			"Расчёт вообще не начнётся, пока анкету не заполнят как минимум три человека.\n\n" +
			"Ссылка рассчитана на одного человека и работает один раз.\n",
	}
}
