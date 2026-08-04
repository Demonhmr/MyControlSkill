package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"mycontrolskill/internal/domain"
)

// Response — заполненная анкета. Связи с приглашением нет: см. комментарий
// к таблице response в миграции 0002.
type Response struct {
	ID           string
	AssessmentID string
	Role         domain.Role
	Tenure       domain.Tenure
	SubmittedAt  time.Time
}

// Counts — сколько анкет собрано в раунде.
type Counts struct {
	External int
	Self     int
}

// Ready сообщает, набралось ли внешних анкет на расчёт профиля.
func (c Counts) Ready() bool { return c.External >= domain.MinRespondents }

// SubmitByToken принимает анкету по ссылке-приглашению.
//
// Роль берётся из приглашения, а не из присланных данных: её назначает
// руководитель, когда зовёт респондента, и подменять её отправителю нельзя.
// Срок наблюдения, наоборот, знает только сам респондент.
//
// Приглашение погашается в той же транзакции, что и вставка ответа, но
// ссылки между ними не остаётся — иначе оценки связывались бы с почтой
// респондента.
func (s *Store) SubmitByToken(ctx context.Context, token string, sub domain.Submission) (Response, error) {
	// Транзакция читает приглашение и тут же его гасит, поэтому идёт через
	// общую очередь записи — см. Store.writeMu.
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Response{}, fmt.Errorf("начало транзакции: %w", err)
	}
	defer tx.Rollback()

	var inviteID, assessmentID, role string
	var usedAt sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT id, assessment_id, role, used_at FROM invite WHERE token_hash = ?`,
		hashToken(token)).Scan(&inviteID, &assessmentID, &role, &usedAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Response{}, ErrNotFound
	case err != nil:
		return Response{}, fmt.Errorf("чтение приглашения: %w", err)
	case usedAt.Valid:
		return Response{}, ErrInviteUsed
	}

	var closedAt sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT closed_at FROM assessment WHERE id = ?`, assessmentID).Scan(&closedAt); err != nil {
		return Response{}, fmt.Errorf("чтение раунда: %w", err)
	}
	if closedAt.Valid {
		return Response{}, ErrAssessmentClosed
	}

	sub.Role = domain.Role(role)
	if err := sub.Validate(); err != nil {
		return Response{}, fmt.Errorf("%w: %w", ErrInvalidSubmission, err)
	}

	// Условие used_at IS NULL — защита от двух одновременных отправок по
	// одной ссылке: вторая получит ноль изменённых строк.
	res, err := tx.ExecContext(ctx,
		`UPDATE invite SET used_at = ? WHERE id = ? AND used_at IS NULL`, now(), inviteID)
	if err != nil {
		return Response{}, fmt.Errorf("погашение приглашения: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return Response{}, fmt.Errorf("погашение приглашения: %w", err)
	} else if n == 0 {
		return Response{}, ErrInviteUsed
	}

	responseID, err := newID()
	if err != nil {
		return Response{}, err
	}
	submitted := now()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO response (id, assessment_id, role, tenure, submitted_at) VALUES (?, ?, ?, ?, ?)`,
		responseID, assessmentID, string(sub.Role), string(sub.Tenure), submitted)
	if err != nil {
		return Response{}, fmt.Errorf("сохранение анкеты: %w", err)
	}

	if err := insertAnswers(ctx, tx, responseID, sub); err != nil {
		return Response{}, err
	}

	if err := tx.Commit(); err != nil {
		return Response{}, fmt.Errorf("фиксация анкеты: %w", err)
	}

	at, err := parseTime(submitted)
	if err != nil {
		return Response{}, err
	}
	return Response{
		ID:           responseID,
		AssessmentID: assessmentID,
		Role:         sub.Role,
		Tenure:       sub.Tenure,
		SubmittedAt:  at,
	}, nil
}

func insertAnswers(ctx context.Context, tx *sql.Tx, responseID string, sub domain.Submission) error {
	if len(sub.Answers) > 0 {
		stmt, err := tx.PrepareContext(ctx,
			`INSERT INTO answer (response_id, kind, code, item_index, value) VALUES (?, ?, ?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("подготовка вставки оценок: %w", err)
		}
		defer stmt.Close()

		for _, a := range sub.Answers {
			var value any
			if a.Value != nil {
				value = *a.Value
			}
			if _, err := stmt.ExecContext(ctx, responseID, string(a.Kind), a.Code, a.ItemIndex, value); err != nil {
				return fmt.Errorf("сохранение оценки %s/%s: %w", a.Kind, a.Code, err)
			}
		}
	}

	for _, o := range sub.OpenAnswers {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO open_answer (response_id, question_index, text) VALUES (?, ?, ?)`,
			responseID, o.QuestionIndex, o.Text); err != nil {
			return fmt.Errorf("сохранение открытого ответа %d: %w", o.QuestionIndex, err)
		}
	}
	return nil
}

// CountResponses считает собранные анкеты раунда.
func (s *Store) CountResponses(ctx context.Context, assessmentID string) (Counts, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT role, count(*) FROM response WHERE assessment_id = ? GROUP BY role`, assessmentID)
	if err != nil {
		return Counts{}, fmt.Errorf("подсчёт анкет: %w", err)
	}
	defer rows.Close()

	var c Counts
	for rows.Next() {
		var role string
		var n int
		if err := rows.Scan(&role, &n); err != nil {
			return Counts{}, fmt.Errorf("подсчёт анкет: %w", err)
		}
		if domain.Role(role).IsExternal() {
			c.External += n
		} else {
			c.Self += n
		}
	}
	return c, rows.Err()
}

// CountOpenAnswers считает сохранённые ответы на открытые вопросы раунда.
func (s *Store) CountOpenAnswers(ctx context.Context, assessmentID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM open_answer o
		   JOIN response r ON r.id = o.response_id
		  WHERE r.assessment_id = ?`, assessmentID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("подсчёт открытых ответов: %w", err)
	}
	return n, nil
}

// ResponsesForScoring отдаёт анкеты раунда для расчёта профиля.
//
// Это единственный способ достать оценки целиком, и наружу его результат
// уходить не должен: клиент получает только агрегат, иначе руководитель
// восстановит, кто как отвечал.
//
// Порядок по rowid — это порядок поступления анкет. Он важен не сам по себе,
// а тем, что строки одной анкеты идут подряд: сборка опирается на смену
// идентификатора.
func (s *Store) ResponsesForScoring(ctx context.Context, assessmentID string) ([]domain.ScoredResponse, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT r.id, r.role, a.kind, a.code, a.item_index, a.value
		   FROM response r
		   LEFT JOIN answer a ON a.response_id = r.id
		  WHERE r.assessment_id = ?
		  ORDER BY r.rowid`, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("чтение анкет: %w", err)
	}
	defer rows.Close()

	var (
		out       []domain.ScoredResponse
		currentID string
	)
	for rows.Next() {
		var (
			id, role  string
			kind      sql.NullString
			code      sql.NullString
			itemIndex sql.NullInt64
			value     sql.NullInt64
		)
		if err := rows.Scan(&id, &role, &kind, &code, &itemIndex, &value); err != nil {
			return nil, fmt.Errorf("чтение анкет: %w", err)
		}

		if id != currentID {
			out = append(out, domain.ScoredResponse{Role: domain.Role(role)})
			currentID = id
		}
		// LEFT JOIN: анкета без единой оценки даёт строку с пустыми полями.
		if !kind.Valid {
			continue
		}

		answer := domain.Answer{
			Kind:      domain.Kind(kind.String),
			Code:      code.String,
			ItemIndex: int(itemIndex.Int64),
		}
		if value.Valid {
			v := int(value.Int64)
			answer.Value = &v
		}
		last := &out[len(out)-1]
		last.Answers = append(last.Answers, answer)
	}
	return out, rows.Err()
}
