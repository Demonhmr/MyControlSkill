// Пакет config читает настройки сервера из переменных окружения.
//
// Файлов конфигурации сознательно нет: сервер разворачивается как один
// бинарник под systemd, а окружение задаётся юнитом.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config — настройки экземпляра сервера.
type Config struct {
	// Addr — адрес прослушивания, например ":8080".
	Addr string
	// DBPath — путь к файлу SQLite.
	DBPath string
	// StaticDir — каталог со сборкой Vite (app/dist).
	StaticDir string
	// BaseURL — внешний адрес сервиса. Нужен для ссылок-приглашений
	// респондентам: письмо уходит наружу, поэтому относительный путь
	// не подходит.
	BaseURL string
	// ShutdownTimeout — сколько ждать завершения активных запросов.
	ShutdownTimeout time.Duration

	// SMTP — почтовый сервер для ссылок входа и приглашений. Если хост
	// пуст, письма не отправляются, а ссылки пишутся в лог.
	SMTP SMTP

	// RetentionDays — через сколько дней раунды 360° удаляются. Ноль
	// означает «хранить бессрочно».
	//
	// Удаляются именно раунды, а не аккаунты: собранная обратная связь
	// стареет и перестаёт что-либо значить, а личный кабинет — нет.
	RetentionDays int

	// Registration — кому разрешено заводить аккаунт. Пустой список
	// означает, что завести его может любой, кто знает адрес сервиса.
	Registration Registration

	// TrustProxy разрешает брать адрес клиента из X-Forwarded-For.
	//
	// Включать только когда перед сервисом действительно стоит обратный
	// прокси. Без прокси заголовок присылает сам клиент, и ограничение
	// частоты по адресу обходится его подделкой.
	TrustProxy bool
}

// SMTP — настройки почтового сервера.
type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	// From — адрес отправителя, допускается вид «Имя <box@example.com>».
	From string
	// Security — starttls, tls или none.
	Security string
}

// Enabled сообщает, настроена ли отправка писем.
func (s SMTP) Enabled() bool { return s.Host != "" }

// SecureCookies сообщает, ставить ли флаг Secure у cookie сессии.
//
// Выводится из схемы BaseURL, а не задаётся отдельно: держать два
// согласованных между собой переключателя — лишний повод их рассинхронить.
// Локальная разработка идёт по http без BaseURL, и там браузер Secure-cookie
// просто не сохранит.
func (c Config) SecureCookies() bool {
	return strings.HasPrefix(c.BaseURL, "https://")
}

const (
	defaultAddr            = ":8080"
	defaultSMTPPort        = 587
	defaultSMTPSecurity    = "starttls"
	defaultDBPath          = "data/mycontrolskill.db"
	defaultStaticDir       = "app/dist"
	defaultShutdownTimeout = 15 * time.Second
)

// Load собирает конфигурацию из окружения и проверяет её.
func Load() (Config, error) {
	port, err := strconv.Atoi(env("MCS_SMTP_PORT", strconv.Itoa(defaultSMTPPort)))
	if err != nil {
		return Config{}, fmt.Errorf("MCS_SMTP_PORT должен быть числом: %w", err)
	}

	retention, err := strconv.Atoi(env("MCS_RETENTION_DAYS", "0"))
	if err != nil {
		return Config{}, fmt.Errorf("MCS_RETENTION_DAYS должен быть числом: %w", err)
	}
	if retention < 0 {
		return Config{}, fmt.Errorf("MCS_RETENTION_DAYS не может быть отрицательным")
	}

	c := Config{
		Addr:            env("MCS_ADDR", defaultAddr),
		DBPath:          env("MCS_DB_PATH", defaultDBPath),
		StaticDir:       env("MCS_STATIC_DIR", defaultStaticDir),
		BaseURL:         strings.TrimSuffix(env("MCS_BASE_URL", ""), "/"),
		ShutdownTimeout: defaultShutdownTimeout,
		TrustProxy:      envBool("MCS_TRUST_PROXY"),
		RetentionDays:   retention,
		Registration: Registration{
			Emails:  parseList(env("MCS_ALLOWED_EMAILS", ""), false),
			Domains: parseList(env("MCS_ALLOWED_DOMAINS", ""), true),
		},
		SMTP: SMTP{
			Host:     env("MCS_SMTP_HOST", ""),
			Port:     port,
			Username: env("MCS_SMTP_USER", ""),
			Password: env("MCS_SMTP_PASSWORD", ""),
			From:     env("MCS_SMTP_FROM", ""),
			Security: env("MCS_SMTP_SECURITY", defaultSMTPSecurity),
		},
	}
	if err := c.validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func (c Config) validate() error {
	if c.Addr == "" {
		return fmt.Errorf("MCS_ADDR пуст")
	}
	if c.DBPath == "" {
		return fmt.Errorf("MCS_DB_PATH пуст")
	}
	if c.BaseURL != "" && !strings.HasPrefix(c.BaseURL, "http://") && !strings.HasPrefix(c.BaseURL, "https://") {
		return fmt.Errorf("MCS_BASE_URL должен начинаться с http:// или https://, получено %q", c.BaseURL)
	}

	if c.SMTP.Enabled() {
		// Отправитель обязателен: без него письма отвергнет первый же
		// приличный почтовый сервер, а узнать об этом хочется при старте,
		// а не при первой попытке входа.
		if c.SMTP.From == "" {
			return fmt.Errorf("MCS_SMTP_HOST задан, но MCS_SMTP_FROM пуст")
		}
		if c.BaseURL == "" {
			// Ссылку в письме собрать не из чего: заголовки запроса тут не
			// помогут, письмо уходит наружу.
			return fmt.Errorf("MCS_SMTP_HOST задан, но MCS_BASE_URL пуст — ссылки в письмах будут нерабочими")
		}
	}
	return nil
}

// envBool читает переключатель. Истина — только явное «да»: опечатка в
// значении не должна молча включать доверие к заголовкам.
func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "да":
		return true
	}
	return false
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
