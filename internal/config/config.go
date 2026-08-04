// Пакет config читает настройки сервера из переменных окружения.
//
// Файлов конфигурации сознательно нет: сервер разворачивается как один
// бинарник под systemd, а окружение задаётся юнитом.
package config

import (
	"fmt"
	"os"
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
}

const (
	defaultAddr            = ":8080"
	defaultDBPath          = "data/mycontrolskill.db"
	defaultStaticDir       = "app/dist"
	defaultShutdownTimeout = 15 * time.Second
)

// Load собирает конфигурацию из окружения и проверяет её.
func Load() (Config, error) {
	c := Config{
		Addr:            env("MCS_ADDR", defaultAddr),
		DBPath:          env("MCS_DB_PATH", defaultDBPath),
		StaticDir:       env("MCS_STATIC_DIR", defaultStaticDir),
		BaseURL:         strings.TrimSuffix(env("MCS_BASE_URL", ""), "/"),
		ShutdownTimeout: defaultShutdownTimeout,
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
	return nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
