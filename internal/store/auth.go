package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Сроки жизни. Ссылка для входа короткая: она уходит по почте и живёт в
// чужом почтовом ящике сколько угодно долго. Сессия длинная, чтобы
// руководитель не логинился на каждый заход.
const (
	LoginTokenTTL = 15 * time.Minute
	SessionTTL    = 30 * 24 * time.Hour
)

// Session — активная сессия руководителя.
type Session struct {
	ID        string
	LeaderID  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// CreateLoginToken выдаёт одноразовую ссылку для входа на указанную почту.
//
// Возвращается сам токен — он существует единственный раз, дальше живёт
// только в письме. В базе лежит хэш.
func (s *Store) CreateLoginToken(ctx context.Context, email string) (string, error) {
	return s.createLoginToken(ctx, email, LoginTokenTTL)
}

// createLoginToken принимает срок жизни отдельным аргументом: тестам нужно
// уметь создавать уже протухшую ссылку.
func (s *Store) createLoginToken(ctx context.Context, email string, ttl time.Duration) (string, error) {
	email = normalizeEmail(email)
	if email == "" {
		return "", fmt.Errorf("почта не указана")
	}

	id, err := newID()
	if err != nil {
		return "", err
	}
	token, err := newToken()
	if err != nil {
		return "", err
	}

	expires := time.Now().UTC().Add(ttl).Format(timeLayout)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO login_token (id, token_hash, email, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		id, hashToken(token), email, now(), expires)
	if err != nil {
		return "", fmt.Errorf("создание ссылки для входа: %w", err)
	}
	return token, nil
}

// ConsumeLoginToken гасит ссылку и возвращает аккаунт, создавая его при
// первом входе.
//
// Регистрации как отдельного шага нет: первый переход по ссылке и есть
// создание аккаунта. Для пилота этого достаточно, но означает, что завести
// аккаунт может любой, кто знает адрес сервиса, — ограничение по списку
// приглашённых появится вместе с продажами.
func (s *Store) ConsumeLoginToken(ctx context.Context, token string) (Leader, error) {
	// Чтение с последующей записью — через общую очередь, см. Store.writeMu.
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Leader{}, fmt.Errorf("начало транзакции: %w", err)
	}
	defer tx.Rollback()

	var id, email, expires string
	var usedAt sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT id, email, expires_at, used_at FROM login_token WHERE token_hash = ?`,
		hashToken(token)).Scan(&id, &email, &expires, &usedAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Leader{}, ErrNotFound
	case err != nil:
		return Leader{}, fmt.Errorf("чтение ссылки для входа: %w", err)
	case usedAt.Valid:
		return Leader{}, ErrTokenUsed
	}

	expiresAt, err := parseTime(expires)
	if err != nil {
		return Leader{}, err
	}
	if !time.Now().UTC().Before(expiresAt) {
		return Leader{}, ErrTokenExpired
	}

	// used_at IS NULL в условии — защита от двух одновременных переходов
	// по одной ссылке.
	res, err := tx.ExecContext(ctx,
		`UPDATE login_token SET used_at = ? WHERE id = ? AND used_at IS NULL`, now(), id)
	if err != nil {
		return Leader{}, fmt.Errorf("погашение ссылки: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return Leader{}, fmt.Errorf("погашение ссылки: %w", err)
	} else if n == 0 {
		return Leader{}, ErrTokenUsed
	}

	leader, err := ensureLeaderTx(ctx, tx, email)
	if err != nil {
		return Leader{}, err
	}
	if err := tx.Commit(); err != nil {
		return Leader{}, fmt.Errorf("фиксация входа: %w", err)
	}
	return leader, nil
}

// ensureLeaderTx — та же логика, что в EnsureLeader, но внутри транзакции
// входа: аккаунт и погашение ссылки должны примениться вместе.
func ensureLeaderTx(ctx context.Context, tx *sql.Tx, email string) (Leader, error) {
	var l Leader
	var created string
	err := tx.QueryRowContext(ctx,
		`SELECT id, email, name, created_at FROM leader WHERE email = ?`, email).
		Scan(&l.ID, &l.Email, &l.Name, &created)

	if errors.Is(err, sql.ErrNoRows) {
		id, err := newID()
		if err != nil {
			return Leader{}, err
		}
		created = now()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO leader (id, email, name, created_at) VALUES (?, ?, '', ?)`,
			id, email, created); err != nil {
			return Leader{}, fmt.Errorf("создание руководителя: %w", err)
		}
		l = Leader{ID: id, Email: email}
	} else if err != nil {
		return Leader{}, fmt.Errorf("чтение руководителя: %w", err)
	}

	at, err := parseTime(created)
	if err != nil {
		return Leader{}, err
	}
	l.CreatedAt = at
	return l, nil
}

// CreateSession заводит сессию и возвращает токен для cookie.
func (s *Store) CreateSession(ctx context.Context, leaderID string) (string, Session, error) {
	return s.createSession(ctx, leaderID, SessionTTL)
}

// createSession принимает срок жизни отдельным аргументом: тестам нужно
// уметь создавать уже протухшую сессию.
func (s *Store) createSession(ctx context.Context, leaderID string, ttl time.Duration) (string, Session, error) {
	id, err := newID()
	if err != nil {
		return "", Session{}, err
	}
	token, err := newToken()
	if err != nil {
		return "", Session{}, err
	}

	createdAt := time.Now().UTC()
	expiresAt := createdAt.Add(ttl)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO session (id, token_hash, leader_id, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		id, hashToken(token), leaderID, createdAt.Format(timeLayout), expiresAt.Format(timeLayout))
	if err != nil {
		return "", Session{}, fmt.Errorf("создание сессии: %w", err)
	}

	return token, Session{
		ID:        id,
		LeaderID:  leaderID,
		CreatedAt: createdAt.Truncate(time.Millisecond),
		ExpiresAt: expiresAt.Truncate(time.Millisecond),
	}, nil
}

// LeaderBySession возвращает владельца сессии по токену из cookie.
func (s *Store) LeaderBySession(ctx context.Context, token string) (Leader, error) {
	var l Leader
	var created, expires string
	err := s.db.QueryRowContext(ctx,
		`SELECT l.id, l.email, l.name, l.created_at, s.expires_at
		   FROM session s JOIN leader l ON l.id = s.leader_id
		  WHERE s.token_hash = ?`, hashToken(token)).
		Scan(&l.ID, &l.Email, &l.Name, &created, &expires)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Leader{}, ErrNotFound
	case err != nil:
		return Leader{}, fmt.Errorf("чтение сессии: %w", err)
	}

	expiresAt, err := parseTime(expires)
	if err != nil {
		return Leader{}, err
	}
	if !time.Now().UTC().Before(expiresAt) {
		return Leader{}, ErrTokenExpired
	}

	at, err := parseTime(created)
	if err != nil {
		return Leader{}, err
	}
	l.CreatedAt = at
	return l, nil
}

// DeleteSession завершает сессию. Отсутствие сессии ошибкой не считается:
// выход должен срабатывать всегда.
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM session WHERE token_hash = ?`, hashToken(token)); err != nil {
		return fmt.Errorf("удаление сессии: %w", err)
	}
	return nil
}

// PurgeExpiredAuth удаляет протухшие ссылки и сессии.
//
// Отдельного планировщика ради этого не заводим: вызов при входе достаточно
// част, а строк тут немного.
func (s *Store) PurgeExpiredAuth(ctx context.Context) error {
	cutoff := now()
	if _, err := s.db.ExecContext(ctx, `DELETE FROM login_token WHERE expires_at <= ?`, cutoff); err != nil {
		return fmt.Errorf("чистка ссылок для входа: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM session WHERE expires_at <= ?`, cutoff); err != nil {
		return fmt.Errorf("чистка сессий: %w", err)
	}
	return nil
}
