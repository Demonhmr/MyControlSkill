package store

import "errors"

var (
	// ErrNotFound — записи с такими признаками нет.
	ErrNotFound = errors.New("запись не найдена")
	// ErrInviteUsed — по этой ссылке анкету уже отправили.
	ErrInviteUsed = errors.New("приглашение уже использовано")
	// ErrAssessmentClosed — раунд закрыт, новые анкеты не принимаются.
	ErrAssessmentClosed = errors.New("раунд оценки закрыт")
	// ErrTokenUsed — по этой ссылке для входа уже заходили.
	ErrTokenUsed = errors.New("ссылка для входа уже использована")
	// ErrTokenExpired — срок жизни ссылки или сессии истёк.
	ErrTokenExpired = errors.New("срок действия истёк")
	// ErrRegistrationClosed — по этому адресу нельзя завести аккаунт.
	//
	// Отличается от ErrNotFound: ссылка была настоящей, но заводить по ней
	// аккаунт не разрешено списком допущенных.
	ErrRegistrationClosed = errors.New("заведение аккаунта по этому адресу не разрешено")
	// ErrInvalidSubmission — присланная анкета не прошла проверку.
	//
	// Отдельная ошибка, чтобы HTTP-слой отличал вину отправителя от сбоя
	// базы: иначе упавший SQLite выглядел бы для клиента как «поправьте
	// анкету», а в логах не осталось бы ничего.
	ErrInvalidSubmission = errors.New("анкета не прошла проверку")
)
