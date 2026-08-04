// Пакет store отвечает за подключение к SQLite и миграции схемы.
//
// Драйвер — modernc.org/sqlite (реализация на чистом Go). Именно он, а не
// mattn/go-sqlite3: последний требует CGO, а сборка проекта идёт с
// CGO_ENABLED=0 ради статических бинарников и кросс-компиляции под Windows
// (см. scripts/build.sh).
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Open открывает базу по пути path, создавая каталог при необходимости,
// и применяет все непрогнанные миграции.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("создание каталога для БД: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("открытие БД: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("проверка подключения к БД: %w", err)
	}

	// SQLite сериализует запись независимо от пула, поэтому большой пул
	// смысла не имеет; busy_timeout в DSN гасит гонки за блокировку.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	if err := Migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// dsn собирает строку подключения. Прагмы задаются на каждое соединение
// пула, поэтому передаются через DSN, а не отдельным запросом.
func dsn(path string) string {
	q := url.Values{}
	// WAL: читатели не блокируют писателя — важно, когда несколько
	// респондентов отправляют анкеты одновременно.
	q.Add("_pragma", "journal_mode(WAL)")
	// Ждать освобождения блокировки вместо немедленного SQLITE_BUSY.
	q.Add("_pragma", "busy_timeout(5000)")
	// Внешние ключи в SQLite выключены по умолчанию.
	q.Add("_pragma", "foreign_keys(ON)")
	return "file:" + path + "?" + q.Encode()
}

// Migrate применяет миграции из каталога migrations по возрастанию имени.
// Повторный вызов ничего не делает: применённые версии отмечены в
// schema_migrations.
func Migrate(ctx context.Context, db *sql.DB) error {
	const createTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version    TEXT PRIMARY KEY,
	applied_at TEXT NOT NULL DEFAULT (datetime('now'))
)`
	if _, err := db.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("создание schema_migrations: %w", err)
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	names, err := migrationNames()
	if err != nil {
		return err
	}

	for _, name := range names {
		if applied[name] {
			continue
		}
		body, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("чтение миграции %s: %w", name, err)
		}
		if err := applyOne(ctx, db, name, string(body)); err != nil {
			return err
		}
	}
	return nil
}

// applyOne прогоняет одну миграцию вместе с отметкой о её применении в
// общей транзакции: наполовину применённых миграций быть не должно.
func applyOne(ctx context.Context, db *sql.DB, name, body string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("начало транзакции для %s: %w", name, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, body); err != nil {
		return fmt.Errorf("применение миграции %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, name); err != nil {
		return fmt.Errorf("отметка миграции %s: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("фиксация миграции %s: %w", name, err)
	}
	return nil
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("чтение schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("чтение версии миграции: %w", err)
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func migrationNames() ([]string, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("чтение каталога миграций: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}
