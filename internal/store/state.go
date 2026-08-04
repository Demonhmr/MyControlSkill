package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Reflection — запись из тренажёра или пульс-трекера.
type Reflection struct {
	ID        string
	Code      string
	Text      string
	CreatedAt time.Time
}

// LoadLeaderState читает рабочее состояние экранов руководителя.
// Для нового аккаунта возвращает пустой объект, а не ошибку.
func (s *Store) LoadLeaderState(ctx context.Context, leaderID string) (json.RawMessage, error) {
	var data string
	err := s.db.QueryRowContext(ctx,
		`SELECT data FROM leader_state WHERE leader_id = ?`, leaderID).Scan(&data)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return json.RawMessage("{}"), nil
	case err != nil:
		return nil, fmt.Errorf("чтение состояния: %w", err)
	}
	return json.RawMessage(data), nil
}

// SaveLeaderState перезаписывает рабочее состояние целиком.
func (s *Store) SaveLeaderState(ctx context.Context, leaderID string, data json.RawMessage) error {
	// Проверка до записи: битый JSON в базе сломал бы чтение всем
	// последующим запросам, а поймать это здесь дёшево.
	if !json.Valid(data) {
		return fmt.Errorf("состояние не является корректным JSON")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO leader_state (leader_id, data, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT (leader_id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		leaderID, string(data), now())
	if err != nil {
		return fmt.Errorf("сохранение состояния: %w", err)
	}
	return nil
}

// AddReflection добавляет запись в дневник практики.
func (s *Store) AddReflection(ctx context.Context, leaderID, code, text string) (Reflection, error) {
	id, err := newID()
	if err != nil {
		return Reflection{}, err
	}
	created := now()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO reflection (id, leader_id, code, text, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, leaderID, code, text, created)
	if err != nil {
		return Reflection{}, fmt.Errorf("сохранение записи: %w", err)
	}

	at, err := parseTime(created)
	if err != nil {
		return Reflection{}, err
	}
	return Reflection{ID: id, Code: code, Text: text, CreatedAt: at}, nil
}

// Reflections отдаёт записи руководителя, свежие первыми.
// limit <= 0 означает «все».
func (s *Store) Reflections(ctx context.Context, leaderID string, limit int) ([]Reflection, error) {
	// rowid вторым ключом — записи одной миллисекунды иначе перемешались бы:
	// идентификаторы случайны и порядка не задают.
	query := `SELECT id, code, text, created_at FROM reflection
	           WHERE leader_id = ? ORDER BY created_at DESC, rowid DESC`
	args := []any{leaderID}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("чтение записей: %w", err)
	}
	defer rows.Close()

	var out []Reflection
	for rows.Next() {
		var r Reflection
		var created string
		if err := rows.Scan(&r.ID, &r.Code, &r.Text, &created); err != nil {
			return nil, fmt.Errorf("чтение записи: %w", err)
		}
		t, err := parseTime(created)
		if err != nil {
			return nil, err
		}
		r.CreatedAt = t
		out = append(out, r)
	}
	return out, rows.Err()
}
