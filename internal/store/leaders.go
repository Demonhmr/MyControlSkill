package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Leader — аккаунт руководителя.
type Leader struct {
	ID        string
	Email     string
	Name      string
	CreatedAt time.Time
}

// EnsureLeader возвращает аккаунт по почте, создавая его при первом входе.
//
// Отдельной регистрации нет: вход по одноразовой ссылке, и первый переход
// по ней и есть создание аккаунта.
func (s *Store) EnsureLeader(ctx context.Context, email, name string) (Leader, error) {
	email = normalizeEmail(email)
	if email == "" {
		return Leader{}, fmt.Errorf("почта не указана")
	}

	if l, err := s.LeaderByEmail(ctx, email); err == nil {
		return l, nil
	} else if !errors.Is(err, ErrNotFound) {
		return Leader{}, err
	}

	id, err := newID()
	if err != nil {
		return Leader{}, err
	}
	created := now()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO leader (id, email, name, created_at) VALUES (?, ?, ?, ?)`,
		id, email, name, created)
	if err != nil {
		return Leader{}, fmt.Errorf("создание руководителя: %w", err)
	}
	return s.LeaderByEmail(ctx, email)
}

// LeaderByEmail ищет аккаунт по почте.
func (s *Store) LeaderByEmail(ctx context.Context, email string) (Leader, error) {
	return s.scanLeader(s.db.QueryRowContext(ctx,
		`SELECT id, email, name, created_at FROM leader WHERE email = ?`, normalizeEmail(email)))
}

// LeaderByAssessment возвращает владельца раунда.
//
// Нужен экрану респондента: тот заполняет анкету по ссылке и должен видеть,
// кого именно оценивает.
func (s *Store) LeaderByAssessment(ctx context.Context, assessmentID string) (Leader, error) {
	return s.scanLeader(s.db.QueryRowContext(ctx,
		`SELECT l.id, l.email, l.name, l.created_at
		   FROM assessment a JOIN leader l ON l.id = a.leader_id
		  WHERE a.id = ?`, assessmentID))
}

// LeaderByID ищет аккаунт по идентификатору.
func (s *Store) LeaderByID(ctx context.Context, id string) (Leader, error) {
	return s.scanLeader(s.db.QueryRowContext(ctx,
		`SELECT id, email, name, created_at FROM leader WHERE id = ?`, id))
}

func (s *Store) scanLeader(row *sql.Row) (Leader, error) {
	var l Leader
	var created string
	switch err := row.Scan(&l.ID, &l.Email, &l.Name, &created); {
	case errors.Is(err, sql.ErrNoRows):
		return Leader{}, ErrNotFound
	case err != nil:
		return Leader{}, fmt.Errorf("чтение руководителя: %w", err)
	}

	t, err := parseTime(created)
	if err != nil {
		return Leader{}, err
	}
	l.CreatedAt = t
	return l, nil
}
