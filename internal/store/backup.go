package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Backup делает согласованную копию базы в destPath.
//
// VACUUM INTO, а не копирование файла: при включённом WAL часть свежих
// изменений лежит рядом, в отдельном журнале, и копия одного лишь .db может
// оказаться без них или вовсе битой. VACUUM INTO выполняется на живой базе,
// не мешает работе и даёт готовый к использованию файл.
//
// Утилита sqlite3 для этого не нужна: она есть не на каждом сервере, а
// драйвер у нас всё равно свой.
func (s *Store) Backup(ctx context.Context, destPath string) error {
	if destPath == "" {
		return fmt.Errorf("не указан путь для копии")
	}

	// VACUUM INTO отказывается писать в существующий файл, но сообщение
	// драйвера об этом невнятное — лучше сказать прямо.
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("файл %s уже существует", destPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("проверка пути для копии: %w", err)
	}

	if dir := filepath.Dir(destPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("создание каталога для копии: %w", err)
		}
	}

	// Путь подставляется в текст запроса: VACUUM INTO не принимает
	// параметры. Одинарные кавычки удваиваются — иначе путь с кавычкой
	// оборвал бы строку и превратился в продолжение запроса.
	quoted := "'" + strings.ReplaceAll(destPath, "'", "''") + "'"
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO "+quoted); err != nil {
		return fmt.Errorf("создание копии базы: %w", err)
	}
	return nil
}
