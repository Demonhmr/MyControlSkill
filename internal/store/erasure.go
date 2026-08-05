package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// DeleteLeader удаляет аккаунт и всё, что к нему привязано.
//
// Дерево уходит каскадами внешних ключей: раунды, приглашения, анкеты,
// оценки, открытые ответы, сессии, участие в организации, рабочее
// состояние, записи тренажёра и журнал согласий.
//
// Отдельно приходится чистить login_token: он привязан к адресу почты, а не
// к аккаунту, потому что существует до его создания. Каскад его не заденет,
// и невыбранные ссылки пережили бы удаление.
//
// Операция необратима: восстановить можно только из копии базы.
func (s *Store) DeleteLeader(ctx context.Context, leaderID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("начало транзакции: %w", err)
	}
	defer tx.Rollback()

	var email string
	err = tx.QueryRowContext(ctx, `SELECT email FROM leader WHERE id = ?`, leaderID).Scan(&email)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrNotFound
	case err != nil:
		return fmt.Errorf("чтение руководителя: %w", err)
	}

	// Организация, где этот человек был единственным эйчаром, останется без
	// эйчара и станет никому не видна. Удаляем её вместе с ним: состав без
	// доступа — мусор, а чужие аккаунты при этом не трогаются, у них лишь
	// пропадает участие.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM org WHERE id IN (
		     SELECT org_id FROM org_member WHERE leader_id = ? AND role = 'hr'
		 ) AND id NOT IN (
		     SELECT org_id FROM org_member WHERE role = 'hr' AND leader_id <> ?
		 )`, leaderID, leaderID); err != nil {
		return fmt.Errorf("удаление осиротевшей организации: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM login_token WHERE email = ?`, email); err != nil {
		return fmt.Errorf("удаление ссылок для входа: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM leader WHERE id = ?`, leaderID); err != nil {
		return fmt.Errorf("удаление руководителя: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("фиксация удаления: %w", err)
	}
	return nil
}

// DeleteAssessment удаляет раунд вместе со всеми собранными по нему анкетами.
func (s *Store) DeleteAssessment(ctx context.Context, assessmentID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	res, err := s.db.ExecContext(ctx, `DELETE FROM assessment WHERE id = ?`, assessmentID)
	if err != nil {
		return fmt.Errorf("удаление раунда: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("удаление раунда: %w", err)
	} else if n == 0 {
		return ErrNotFound
	}
	return nil
}

// PurgeOlderThan удаляет раунды, начатые раньше cutoff, и возвращает их число.
//
// Удаляются именно раунды, а не аккаунты: собранная обратная связь стареет и
// перестаёт что-либо значить, а личный кабинет — нет.
func (s *Store) PurgeOlderThan(ctx context.Context, cutoff time.Time) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	res, err := s.db.ExecContext(ctx,
		`DELETE FROM assessment WHERE created_at < ?`, cutoff.UTC().Format(timeLayout))
	if err != nil {
		return 0, fmt.Errorf("чистка старых раундов: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("чистка старых раундов: %w", err)
	}
	return int(n), nil
}
