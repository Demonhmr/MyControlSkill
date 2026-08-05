package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// OrgRole — роль участника организации.
type OrgRole string

const (
	// OrgRoleHR видит сводку по всем участникам.
	OrgRoleHR OrgRole = "hr"
	// OrgRoleLeader видит только свои раунды.
	OrgRoleLeader OrgRole = "leader"
)

func (r OrgRole) Valid() bool { return r == OrgRoleHR || r == OrgRoleLeader }

// Org — организация.
type Org struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// Member — участник организации вместе с его аккаунтом.
type Member struct {
	LeaderID string
	Email    string
	Name     string
	Role     OrgRole
	JoinedAt time.Time
	// ProfileConsentAt — когда человек разрешил показывать свой профиль
	// HR-службе. nil означает, что согласия нет: молчание согласием не
	// считается, и по умолчанию эйчар чисел не видит.
	ProfileConsentAt *time.Time
}

// ProfileConsentGranted сообщает, разрешён ли показ профиля HR-службе.
func (m Member) ProfileConsentGranted() bool { return m.ProfileConsentAt != nil }

// ErrAlreadyInOrg — руководитель уже состоит в организации.
var ErrAlreadyInOrg = errors.New("руководитель уже состоит в организации")

// CreateOrg заводит организацию; создатель становится в ней эйчаром.
func (s *Store) CreateOrg(ctx context.Context, name, leaderID string) (Org, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Org{}, fmt.Errorf("не указано название организации")
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Org{}, fmt.Errorf("начало транзакции: %w", err)
	}
	defer tx.Rollback()

	var existing string
	err = tx.QueryRowContext(ctx, `SELECT org_id FROM org_member WHERE leader_id = ?`, leaderID).Scan(&existing)
	switch {
	case err == nil:
		return Org{}, ErrAlreadyInOrg
	case !errors.Is(err, sql.ErrNoRows):
		return Org{}, fmt.Errorf("проверка участия: %w", err)
	}

	id, err := newID()
	if err != nil {
		return Org{}, err
	}
	created := now()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO org (id, name, created_at) VALUES (?, ?, ?)`, id, name, created); err != nil {
		return Org{}, fmt.Errorf("создание организации: %w", err)
	}
	// Создатель сразу эйчар: организация без единого эйчара никому не видна.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO org_member (org_id, leader_id, role, joined_at) VALUES (?, ?, ?, ?)`,
		id, leaderID, string(OrgRoleHR), created); err != nil {
		return Org{}, fmt.Errorf("добавление создателя: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Org{}, fmt.Errorf("фиксация организации: %w", err)
	}

	at, err := parseTime(created)
	if err != nil {
		return Org{}, err
	}
	return Org{ID: id, Name: name, CreatedAt: at}, nil
}

// OrgForLeader возвращает организацию руководителя и его роль в ней.
func (s *Store) OrgForLeader(ctx context.Context, leaderID string) (Org, OrgRole, error) {
	var o Org
	var role, created string
	err := s.db.QueryRowContext(ctx,
		`SELECT o.id, o.name, o.created_at, m.role
		   FROM org_member m JOIN org o ON o.id = m.org_id
		  WHERE m.leader_id = ?`, leaderID).Scan(&o.ID, &o.Name, &created, &role)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Org{}, "", ErrNotFound
	case err != nil:
		return Org{}, "", fmt.Errorf("чтение организации: %w", err)
	}

	at, err := parseTime(created)
	if err != nil {
		return Org{}, "", err
	}
	o.CreatedAt = at
	return o, OrgRole(role), nil
}

// AddOrgMember добавляет руководителя в организацию по адресу почты,
// заводя аккаунт, если его ещё нет.
//
// Аккаунт создаётся заранее намеренно: эйчар собирает состав до того, как
// люди впервые войдут, а первый вход по ссылке подхватит уже готовый.
func (s *Store) AddOrgMember(ctx context.Context, orgID, email string, role OrgRole) (Member, error) {
	if !role.Valid() {
		return Member{}, fmt.Errorf("неизвестная роль %q", role)
	}
	email = normalizeEmail(email)
	if email == "" {
		return Member{}, fmt.Errorf("почта не указана")
	}

	leader, err := s.EnsureLeader(ctx, email, "")
	if err != nil {
		return Member{}, err
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Member{}, fmt.Errorf("начало транзакции: %w", err)
	}
	defer tx.Rollback()

	var existing string
	err = tx.QueryRowContext(ctx, `SELECT org_id FROM org_member WHERE leader_id = ?`, leader.ID).Scan(&existing)
	switch {
	case err == nil:
		// Уже в этой же организации — не ошибка, состав просто не меняется.
		if existing == orgID {
			return s.memberByLeader(ctx, orgID, leader.ID)
		}
		return Member{}, ErrAlreadyInOrg
	case !errors.Is(err, sql.ErrNoRows):
		return Member{}, fmt.Errorf("проверка участия: %w", err)
	}

	joined := now()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO org_member (org_id, leader_id, role, joined_at) VALUES (?, ?, ?, ?)`,
		orgID, leader.ID, string(role), joined); err != nil {
		return Member{}, fmt.Errorf("добавление участника: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Member{}, fmt.Errorf("фиксация участника: %w", err)
	}

	at, err := parseTime(joined)
	if err != nil {
		return Member{}, err
	}
	return Member{LeaderID: leader.ID, Email: leader.Email, Name: leader.Name, Role: role, JoinedAt: at}, nil
}

func (s *Store) memberByLeader(ctx context.Context, orgID, leaderID string) (Member, error) {
	var m Member
	var role, joined string
	var consent sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT l.id, l.email, l.name, m.role, m.joined_at, m.profile_consent_at
		   FROM org_member m JOIN leader l ON l.id = m.leader_id
		  WHERE m.org_id = ? AND m.leader_id = ?`, orgID, leaderID).
		Scan(&m.LeaderID, &m.Email, &m.Name, &role, &joined, &consent)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Member{}, ErrNotFound
	case err != nil:
		return Member{}, fmt.Errorf("чтение участника: %w", err)
	}
	m.Role = OrgRole(role)

	at, err := parseTime(joined)
	if err != nil {
		return Member{}, err
	}
	m.JoinedAt = at

	if consent.Valid {
		ct, err := parseTime(consent.String)
		if err != nil {
			return Member{}, err
		}
		m.ProfileConsentAt = &ct
	}
	return m, nil
}

// OrgMembers перечисляет состав организации.
func (s *Store) OrgMembers(ctx context.Context, orgID string) ([]Member, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT l.id, l.email, l.name, m.role, m.joined_at, m.profile_consent_at
		   FROM org_member m JOIN leader l ON l.id = m.leader_id
		  WHERE m.org_id = ? ORDER BY m.joined_at, m.rowid`, orgID)
	if err != nil {
		return nil, fmt.Errorf("чтение состава: %w", err)
	}
	defer rows.Close()

	var out []Member
	for rows.Next() {
		var m Member
		var role, joined string
		var consent sql.NullString
		if err := rows.Scan(&m.LeaderID, &m.Email, &m.Name, &role, &joined, &consent); err != nil {
			return nil, fmt.Errorf("чтение участника: %w", err)
		}
		m.Role = OrgRole(role)

		at, err := parseTime(joined)
		if err != nil {
			return nil, err
		}
		m.JoinedAt = at

		if consent.Valid {
			ct, err := parseTime(consent.String)
			if err != nil {
				return nil, err
			}
			m.ProfileConsentAt = &ct
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetProfileConsent выдаёт или отзывает согласие на показ профиля HR-службе.
//
// Текущее состояние пишется в org_member, а факт изменения — в журнал.
// Одного текущего состояния мало: отозванное согласие неотличимо от
// никогда не выданного, а спор о том, давал ли человек согласие и когда,
// разрешается только записью.
//
// Повторная выдача момент согласия не сдвигает: человек согласился тогда,
// когда согласился, а не когда последний раз нажал кнопку.
func (s *Store) SetProfileConsent(ctx context.Context, leaderID string, granted bool) (Member, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Member{}, fmt.Errorf("начало транзакции: %w", err)
	}
	defer tx.Rollback()

	var orgID string
	var current sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT org_id, profile_consent_at FROM org_member WHERE leader_id = ?`, leaderID).
		Scan(&orgID, &current)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Member{}, ErrNotFound
	case err != nil:
		return Member{}, fmt.Errorf("чтение участия: %w", err)
	}

	changed := granted != current.Valid
	if changed {
		var value any
		if granted {
			value = now()
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE org_member SET profile_consent_at = ? WHERE leader_id = ?`, value, leaderID); err != nil {
			return Member{}, fmt.Errorf("сохранение согласия: %w", err)
		}

		id, err := newID()
		if err != nil {
			return Member{}, err
		}
		grantedInt := 0
		if granted {
			grantedInt = 1
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO consent_event (id, leader_id, org_id, granted, created_at) VALUES (?, ?, ?, ?, ?)`,
			id, leaderID, orgID, grantedInt, now()); err != nil {
			return Member{}, fmt.Errorf("запись в журнал согласий: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Member{}, fmt.Errorf("фиксация согласия: %w", err)
	}
	return s.memberByLeader(ctx, orgID, leaderID)
}

// ConsentEvent — запись журнала выдачи и отзыва согласия.
type ConsentEvent struct {
	Granted   bool
	CreatedAt time.Time
}

// ConsentHistory отдаёт журнал по руководителю, свежие первыми.
func (s *Store) ConsentHistory(ctx context.Context, leaderID string) ([]ConsentEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT granted, created_at FROM consent_event
		  WHERE leader_id = ? ORDER BY created_at DESC, rowid DESC`, leaderID)
	if err != nil {
		return nil, fmt.Errorf("чтение журнала согласий: %w", err)
	}
	defer rows.Close()

	var out []ConsentEvent
	for rows.Next() {
		var granted int
		var created string
		if err := rows.Scan(&granted, &created); err != nil {
			return nil, fmt.Errorf("чтение записи журнала: %w", err)
		}
		at, err := parseTime(created)
		if err != nil {
			return nil, err
		}
		out = append(out, ConsentEvent{Granted: granted == 1, CreatedAt: at})
	}
	return out, rows.Err()
}

// MemberOf возвращает участие руководителя в организации.
func (s *Store) MemberOf(ctx context.Context, orgID, leaderID string) (Member, error) {
	return s.memberByLeader(ctx, orgID, leaderID)
}

// LatestAssessment возвращает самый свежий раунд руководителя.
func (s *Store) LatestAssessment(ctx context.Context, leaderID string) (Assessment, error) {
	return scanAssessment(s.db.QueryRowContext(ctx,
		`SELECT id, leader_id, title, created_at, closed_at
		   FROM assessment WHERE leader_id = ?
		  ORDER BY created_at DESC, rowid DESC LIMIT 1`, leaderID))
}
