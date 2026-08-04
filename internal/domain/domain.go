// Пакет domain хранит словарь методики: компетенции, деструкторы, роли
// респондентов и границы шкалы.
//
// Это перенос app/src/data/index.js и app/src/data/questionnaire.js на
// серверную сторону. Дублирование намеренное: клиент рисует анкету, сервер
// обязан проверять присланное независимо — иначе достаточно подделать запрос,
// чтобы засорить нормативную базу.
package domain

import (
	"fmt"
	"unicode/utf8"
)

// Границы шкалы оценки. Отдельное значение «не могу оценить» шкалой не
// является и приходит как отсутствие оценки (nil), см. Answer.Value.
const (
	MinScore = 1
	MaxScore = 5
)

// ItemsPerCode — сколько пунктов анкеты приходится на одну компетенцию или
// деструктор. Индексы пунктов идут от нуля.
const ItemsPerCode = 2

// OpenQuestionCount — число открытых вопросов в конце анкеты.
const OpenQuestionCount = 2

// MaxOpenAnswerLength — предел длины свободного ответа в символах.
//
// Ограничение техническое, а не смысловое: развёрнутый ответ на пару
// абзацев укладывается с запасом, а без предела в базу можно залить
// сколько угодно текста.
const MaxOpenAnswerLength = 5000

// MinRespondents — сколько внешних анкет нужно, чтобы считать профиль.
// Ниже этого порога отчёт строился бы на шуме, поэтому сервер его не отдаёт.
// Синхронизировано с MIN_RESPONDENTS в app/src/logic/scoring.js.
const MinRespondents = 3

// Role — кем респондент приходится оцениваемому руководителю.
type Role string

const (
	RoleSubordinate Role = "subordinate"
	RolePeer        Role = "peer"
	RoleManager     Role = "manager"
	RoleSelf        Role = "self"
)

// IsExternal сообщает, идёт ли ответ в расчёт профиля. Самооценка в него
// не входит: она нужна только для поиска слепых зон.
func (r Role) IsExternal() bool { return r != RoleSelf && r.Valid() }

func (r Role) Valid() bool {
	switch r {
	case RoleSubordinate, RolePeer, RoleManager, RoleSelf:
		return true
	}
	return false
}

// Tenure — как долго респондент наблюдает руководителя.
type Tenure string

const (
	TenureLessThan3Months Tenure = "lt3"
	Tenure3To12Months     Tenure = "3to12"
	TenureOver1Year       Tenure = "gt12"
)

func (t Tenure) Valid() bool {
	switch t {
	case TenureLessThan3Months, Tenure3To12Months, TenureOver1Year:
		return true
	}
	return false
}

// Kind различает две шкалы анкеты: сильные стороны и критические зоны.
type Kind string

const (
	KindCompetency Kind = "competency"
	KindDestructor Kind = "destructor"
)

// CompetencyCodes — 19 компетенций модели Zenger Folkman.
var CompetencyCodes = []string{
	"INT", "EXP", "PSA", "INN", "LRN",
	"RES", "GOL", "INI", "DEC", "RSK",
	"COM", "INS", "REL", "DEV", "COL", "DIV",
	"VIS", "CHG", "CUS",
}

// DestructorCodes — 10 деструкторов.
var DestructorCodes = []string{
	"d1", "d2", "d3", "d4", "d5", "d6", "d7", "d8", "d9", "d10",
}

var (
	competencySet = toSet(CompetencyCodes)
	destructorSet = toSet(DestructorCodes)
)

func toSet(codes []string) map[string]bool {
	set := make(map[string]bool, len(codes))
	for _, c := range codes {
		set[c] = true
	}
	return set
}

// ValidCode проверяет, что код принадлежит нужной шкале.
func ValidCode(kind Kind, code string) bool {
	switch kind {
	case KindCompetency:
		return competencySet[code]
	case KindDestructor:
		return destructorSet[code]
	}
	return false
}

// Answer — оценка одного пункта анкеты.
type Answer struct {
	Kind      Kind
	Code      string
	ItemIndex int
	// Value равно nil, если респондент выбрал «не могу оценить». Такие
	// пункты в среднее не идут, но хранятся: их доля показывает, насколько
	// респондент вообще наблюдает руководителя.
	Value *int
}

// Validate проверяет один ответ.
func (a Answer) Validate() error {
	if !ValidCode(a.Kind, a.Code) {
		return fmt.Errorf("неизвестный код %q для шкалы %q", a.Code, a.Kind)
	}
	if a.ItemIndex < 0 || a.ItemIndex >= ItemsPerCode {
		return fmt.Errorf("пункт %d вне диапазона 0..%d для %s", a.ItemIndex, ItemsPerCode-1, a.Code)
	}
	if a.Value != nil && (*a.Value < MinScore || *a.Value > MaxScore) {
		return fmt.Errorf("оценка %d вне шкалы %d..%d для %s", *a.Value, MinScore, MaxScore, a.Code)
	}
	return nil
}

// OpenAnswer — свободный текст на открытый вопрос.
type OpenAnswer struct {
	QuestionIndex int
	Text          string
}

func (o OpenAnswer) Validate() error {
	if o.QuestionIndex < 0 || o.QuestionIndex >= OpenQuestionCount {
		return fmt.Errorf("открытый вопрос %d вне диапазона 0..%d", o.QuestionIndex, OpenQuestionCount-1)
	}
	// Длина в рунах, а не в байтах: ответы на русском, и байтовый предел
	// урезал бы их вдвое против ожидаемого.
	if n := utf8.RuneCountInString(o.Text); n > MaxOpenAnswerLength {
		return fmt.Errorf("ответ на открытый вопрос %d длиной %d символов, предел %d",
			o.QuestionIndex, n, MaxOpenAnswerLength)
	}
	return nil
}

// ScoredResponse — сохранённая анкета в виде, пригодном для расчёта профиля.
// Живёт здесь, а не в store: на неё опирается и хранилище, и скоринг.
type ScoredResponse struct {
	Role    Role
	Answers []Answer
}

// Submission — присланная анкета целиком.
type Submission struct {
	Role        Role
	Tenure      Tenure
	Answers     []Answer
	OpenAnswers []OpenAnswer
}

// Validate проверяет анкету перед записью. Пустая анкета допустима:
// респондент вправе не оценить ни одного пункта, а порог MinRespondents
// всё равно не даст построить отчёт на пустых данных.
func (s Submission) Validate() error {
	if !s.Role.Valid() {
		return fmt.Errorf("неизвестная роль %q", s.Role)
	}
	if !s.Tenure.Valid() {
		return fmt.Errorf("неизвестный срок наблюдения %q", s.Tenure)
	}

	seen := make(map[string]bool, len(s.Answers))
	for _, a := range s.Answers {
		if err := a.Validate(); err != nil {
			return err
		}
		key := fmt.Sprintf("%s/%s/%d", a.Kind, a.Code, a.ItemIndex)
		if seen[key] {
			return fmt.Errorf("пункт %s прислан дважды", key)
		}
		seen[key] = true
	}

	seenOpen := make(map[int]bool, len(s.OpenAnswers))
	for _, o := range s.OpenAnswers {
		if err := o.Validate(); err != nil {
			return err
		}
		if seenOpen[o.QuestionIndex] {
			return fmt.Errorf("открытый вопрос %d прислан дважды", o.QuestionIndex)
		}
		seenOpen[o.QuestionIndex] = true
	}
	return nil
}
