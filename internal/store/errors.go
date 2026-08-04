package store

import "errors"

var (
	// ErrNotFound — записи с такими признаками нет.
	ErrNotFound = errors.New("запись не найдена")
	// ErrInviteUsed — по этой ссылке анкету уже отправили.
	ErrInviteUsed = errors.New("приглашение уже использовано")
	// ErrAssessmentClosed — раунд закрыт, новые анкеты не принимаются.
	ErrAssessmentClosed = errors.New("раунд оценки закрыт")
)
