package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"mycontrolskill/internal/domain"
)

// Assessment — раунд оценки 360°.
type Assessment struct {
	ID        string
	LeaderID  string
	Title     string
	CreatedAt time.Time
	ClosedAt  *time.Time
}

// Closed сообщает, принимает ли раунд новые анкеты.
func (a Assessment) Closed() bool { return a.ClosedAt != nil }

// Invite — приглашение респонденту.
//
// Самого токена здесь нет: он существует ровно один раз, в момент выдачи,
// и дальше живёт только в ссылке у респондента.
type Invite struct {
	ID           string
	AssessmentID string
	Role         domain.Role
	Email        string
	CreatedAt    time.Time
	UsedAt       *time.Time
}

// Used сообщает, отправлена ли анкета по этому приглашению.
func (i Invite) Used() bool { return i.UsedAt != nil }

// CreateAssessment заводит новый раунд для руководителя.
func (s *Store) CreateAssessment(ctx context.Context, leaderID, title string) (Assessment, error) {
	id, err := newID()
	if err != nil {
		return Assessment{}, err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO assessment (id, leader_id, title, created_at) VALUES (?, ?, ?, ?)`,
		id, leaderID, title, now())
	if err != nil {
		return Assessment{}, fmt.Errorf("создание раунда: %w", err)
	}
	return s.AssessmentByID(ctx, id)
}

// AssessmentByID читает раунд.
func (s *Store) AssessmentByID(ctx context.Context, id string) (Assessment, error) {
	return scanAssessment(s.db.QueryRowContext(ctx,
		`SELECT id, leader_id, title, created_at, closed_at FROM assessment WHERE id = ?`, id))
}

// AssessmentsByLeader перечисляет раунды руководителя, свежие первыми.
func (s *Store) AssessmentsByLeader(ctx context.Context, leaderID string) ([]Assessment, error) {
	// rowid вторым ключом — детерминированный разрыв ничьей: метка времени
	// имеет миллисекундную точность, а идентификаторы случайны, и без него
	// раунды одной миллисекунды выстраивались бы в произвольном порядке.
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, leader_id, title, created_at, closed_at
		   FROM assessment WHERE leader_id = ? ORDER BY created_at DESC, rowid DESC`, leaderID)
	if err != nil {
		return nil, fmt.Errorf("чтение раундов: %w", err)
	}
	defer rows.Close()

	var out []Assessment
	for rows.Next() {
		a, err := scanAssessment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CloseAssessment закрывает раунд: новые анкеты по его ссылкам не принимаются.
// Повторное закрытие момент закрытия не сдвигает.
func (s *Store) CloseAssessment(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE assessment SET closed_at = ? WHERE id = ? AND closed_at IS NULL`, now(), id)
	if err != nil {
		return fmt.Errorf("закрытие раунда: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		// Ноль строк — либо раунда нет, либо он уже закрыт; различаем.
		if _, err := s.AssessmentByID(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// CreateInvite выдаёт приглашение и возвращает токен для ссылки.
//
// Токен возвращается единственный раз: в базе лежит только его хэш,
// восстановить ссылку позже нельзя — можно лишь выдать новую.
func (s *Store) CreateInvite(ctx context.Context, assessmentID string, role domain.Role, email string) (Invite, string, error) {
	if !role.Valid() {
		return Invite{}, "", fmt.Errorf("неизвестная роль %q", role)
	}
	a, err := s.AssessmentByID(ctx, assessmentID)
	if err != nil {
		return Invite{}, "", err
	}
	if a.Closed() {
		return Invite{}, "", ErrAssessmentClosed
	}

	id, err := newID()
	if err != nil {
		return Invite{}, "", err
	}
	token, err := newToken()
	if err != nil {
		return Invite{}, "", err
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO invite (id, assessment_id, token_hash, role, email, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, assessmentID, hashToken(token), string(role), normalizeEmail(email), now())
	if err != nil {
		return Invite{}, "", fmt.Errorf("создание приглашения: %w", err)
	}

	inv, err := s.inviteByID(ctx, id)
	if err != nil {
		return Invite{}, "", err
	}
	return inv, token, nil
}

// InviteByToken находит приглашение по токену из ссылки.
func (s *Store) InviteByToken(ctx context.Context, token string) (Invite, error) {
	return scanInvite(s.db.QueryRowContext(ctx,
		`SELECT id, assessment_id, role, email, created_at, used_at
		   FROM invite WHERE token_hash = ?`, hashToken(token)))
}

// InvitesByAssessment перечисляет приглашения раунда — для экрана «кого позвали».
func (s *Store) InvitesByAssessment(ctx context.Context, assessmentID string) ([]Invite, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, assessment_id, role, email, created_at, used_at
		   FROM invite WHERE assessment_id = ? ORDER BY created_at, rowid`, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("чтение приглашений: %w", err)
	}
	defer rows.Close()

	var out []Invite
	for rows.Next() {
		inv, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (s *Store) inviteByID(ctx context.Context, id string) (Invite, error) {
	return scanInvite(s.db.QueryRowContext(ctx,
		`SELECT id, assessment_id, role, email, created_at, used_at
		   FROM invite WHERE id = ?`, id))
}

// scanner покрывает и *sql.Row, и *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanAssessment(row scanner) (Assessment, error) {
	var a Assessment
	var created string
	var closed sql.NullString
	switch err := row.Scan(&a.ID, &a.LeaderID, &a.Title, &created, &closed); {
	case errors.Is(err, sql.ErrNoRows):
		return Assessment{}, ErrNotFound
	case err != nil:
		return Assessment{}, fmt.Errorf("чтение раунда: %w", err)
	}

	t, err := parseTime(created)
	if err != nil {
		return Assessment{}, err
	}
	a.CreatedAt = t

	if closed.Valid {
		ct, err := parseTime(closed.String)
		if err != nil {
			return Assessment{}, err
		}
		a.ClosedAt = &ct
	}
	return a, nil
}

func scanInvite(row scanner) (Invite, error) {
	var i Invite
	var role, created string
	var used sql.NullString
	switch err := row.Scan(&i.ID, &i.AssessmentID, &role, &i.Email, &created, &used); {
	case errors.Is(err, sql.ErrNoRows):
		return Invite{}, ErrNotFound
	case err != nil:
		return Invite{}, fmt.Errorf("чтение приглашения: %w", err)
	}
	i.Role = domain.Role(role)

	t, err := parseTime(created)
	if err != nil {
		return Invite{}, err
	}
	i.CreatedAt = t

	if used.Valid {
		ut, err := parseTime(used.String)
		if err != nil {
			return Invite{}, err
		}
		i.UsedAt = &ut
	}
	return i, nil
}
