// Пакет mail отправляет письма со ссылками.
//
// Реальной отправки по SMTP пока нет: для разработки и локального пилота
// ссылка печатается в лог сервера. Это осознанная заглушка, а не забытый
// код — до боевого запуска её обязательно заменить, иначе войти сможет
// только тот, у кого есть доступ к логам.
package mail

import (
	"context"
	"fmt"
	"log/slog"
)

// Mailer отправляет письма руководителям и респондентам.
type Mailer interface {
	// SendLoginLink отправляет ссылку для входа в личный кабинет.
	SendLoginLink(ctx context.Context, email, link string) error
	// SendInvite отправляет респонденту ссылку на анкету 360°.
	SendInvite(ctx context.Context, email, link, leaderName string) error
}

// LogMailer печатает письма в лог вместо отправки.
type LogMailer struct {
	log *slog.Logger
}

// NewLogMailer создаёт отправщика, пишущий в переданный лог.
func NewLogMailer(log *slog.Logger) *LogMailer {
	return &LogMailer{log: log}
}

func (m *LogMailer) SendLoginLink(ctx context.Context, email, link string) error {
	if email == "" || link == "" {
		return fmt.Errorf("пустой адрес или ссылка")
	}
	// Ссылка в логе — это, по сути, пароль. Отдельная отметка, чтобы такой
	// лог не утёк в общий сборщик по недосмотру.
	m.log.Warn("письмо не отправлено, отправка по SMTP не настроена — ссылка для входа выведена в лог",
		"kind", "login", "email", email, "link", link)
	return nil
}

func (m *LogMailer) SendInvite(ctx context.Context, email, link, leaderName string) error {
	if link == "" {
		return fmt.Errorf("пустая ссылка")
	}
	m.log.Warn("письмо не отправлено, отправка по SMTP не настроена — ссылка на анкету выведена в лог",
		"kind", "invite", "email", email, "leader", leaderName, "link", link)
	return nil
}
