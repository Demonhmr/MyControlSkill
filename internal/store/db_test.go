package store

import (
	"context"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) (context.Context, string) {
	t.Helper()
	return context.Background(), filepath.Join(t.TempDir(), "вложенный", "test.db")
}

func TestOpenСоздаётКаталогИПрименяетМиграции(t *testing.T) {
	ctx, path := openTestDB(t)

	st, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	var n int
	if err := st.db.QueryRowContext(ctx, `SELECT count(*) FROM leader`).Scan(&n); err != nil {
		t.Fatalf("таблица leader недоступна: %v", err)
	}

	// Обе инструкции миграции должны примениться, включая индекс.
	var idx string
	err = st.db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='index' AND name='leader_email_idx'`).Scan(&idx)
	if err != nil {
		t.Fatalf("индекс leader_email_idx не создан: %v", err)
	}
}

func TestMigrateИдемпотентна(t *testing.T) {
	ctx, path := openTestDB(t)

	st, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	// Open уже прогнал миграции; повтор не должен падать на «table exists».
	if err := Migrate(ctx, st.db); err != nil {
		t.Fatalf("повторный Migrate: %v", err)
	}

	var applied int
	if err := st.db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatalf("чтение schema_migrations: %v", err)
	}
	names, err := migrationNames()
	if err != nil {
		t.Fatalf("migrationNames: %v", err)
	}
	if applied != len(names) {
		t.Errorf("применено миграций = %d, файлов = %d", applied, len(names))
	}
}

func TestВнешниеКлючиВключены(t *testing.T) {
	ctx, path := openTestDB(t)

	st, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	// В SQLite foreign_keys выключены по умолчанию, а схема на них
	// рассчитывает — прагма должна доезжать через DSN на каждое соединение.
	var on int
	if err := st.db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&on); err != nil {
		t.Fatalf("чтение PRAGMA foreign_keys: %v", err)
	}
	if on != 1 {
		t.Errorf("foreign_keys = %d, ожидалось 1", on)
	}
}
