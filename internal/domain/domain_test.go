package domain

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

func ptr(v int) *int { return &v }

func TestValidCode(t *testing.T) {
	cases := []struct {
		kind Kind
		code string
		want bool
	}{
		{KindCompetency, "INT", true},
		{KindCompetency, "CUS", true},
		{KindCompetency, "d1", false},  // деструктор не компетенция
		{KindCompetency, "int", false}, // регистр значим
		{KindDestructor, "d10", true},
		{KindDestructor, "d11", false},
		{KindDestructor, "COM", false},
		{Kind("прочее"), "INT", false},
	}
	for _, c := range cases {
		if got := ValidCode(c.kind, c.code); got != c.want {
			t.Errorf("ValidCode(%q, %q) = %v, ожидалось %v", c.kind, c.code, got, c.want)
		}
	}
}

func TestAnswerValidate(t *testing.T) {
	cases := []struct {
		name    string
		answer  Answer
		wantErr bool
	}{
		{"обычная оценка", Answer{KindCompetency, "INT", 0, ptr(3)}, false},
		{"не могу оценить", Answer{KindCompetency, "INT", 1, nil}, false},
		{"нижняя граница шкалы", Answer{KindDestructor, "d1", 0, ptr(MinScore)}, false},
		{"верхняя граница шкалы", Answer{KindDestructor, "d1", 1, ptr(MaxScore)}, false},
		{"ноль вне шкалы", Answer{KindCompetency, "INT", 0, ptr(0)}, true},
		{"шесть вне шкалы", Answer{KindCompetency, "INT", 0, ptr(6)}, true},
		{"пункт вне диапазона", Answer{KindCompetency, "INT", ItemsPerCode, ptr(3)}, true},
		{"отрицательный пункт", Answer{KindCompetency, "INT", -1, ptr(3)}, true},
		{"неизвестный код", Answer{KindCompetency, "XXX", 0, ptr(3)}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.answer.Validate()
			if (err != nil) != c.wantErr {
				t.Errorf("Validate() = %v, ожидалась ошибка: %v", err, c.wantErr)
			}
		})
	}
}

func TestRoleIsExternal(t *testing.T) {
	// В расчёт профиля идут все роли, кроме самооценки.
	for _, r := range []Role{RoleSubordinate, RolePeer, RoleManager} {
		if !r.IsExternal() {
			t.Errorf("роль %q должна считаться внешней", r)
		}
	}
	if RoleSelf.IsExternal() {
		t.Error("самооценка не должна считаться внешним ответом")
	}
	if Role("выдуманная").IsExternal() {
		t.Error("неизвестная роль не должна считаться внешней")
	}
}

func TestSubmissionValidate(t *testing.T) {
	ok := Submission{
		Role:        RolePeer,
		Tenure:      TenureOver1Year,
		Answers:     []Answer{{KindCompetency, "INT", 0, ptr(4)}},
		OpenAnswers: []OpenAnswer{{0, "текст"}},
	}
	if err := ok.Validate(); err != nil {
		t.Fatalf("корректная анкета отклонена: %v", err)
	}

	// Пустая анкета допустима: порог MinRespondents всё равно не даст
	// построить отчёт на пустых данных.
	empty := Submission{Role: RoleSelf, Tenure: TenureLessThan3Months}
	if err := empty.Validate(); err != nil {
		t.Errorf("пустая анкета отклонена: %v", err)
	}

	bad := []struct {
		name string
		sub  Submission
	}{
		{"неизвестная роль", Submission{Role: "начальник", Tenure: TenureOver1Year}},
		{"неизвестный срок", Submission{Role: RolePeer, Tenure: "давно"}},
		{"дубль пункта", Submission{Role: RolePeer, Tenure: TenureOver1Year, Answers: []Answer{
			{KindCompetency, "INT", 0, ptr(4)},
			{KindCompetency, "INT", 0, ptr(5)},
		}}},
		{"дубль открытого вопроса", Submission{Role: RolePeer, Tenure: TenureOver1Year, OpenAnswers: []OpenAnswer{
			{0, "раз"}, {0, "два"},
		}}},
		{"открытый вопрос вне диапазона", Submission{Role: RolePeer, Tenure: TenureOver1Year, OpenAnswers: []OpenAnswer{
			{OpenQuestionCount, "текст"},
		}}},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			if err := c.sub.Validate(); err == nil {
				t.Error("ожидалась ошибка, получено nil")
			}
		})
	}
}

// Словарь продублирован на клиенте и на сервере. Тест ловит расхождение:
// добавленная в JS компетенция без правки Go молча отвергалась бы сервером
// при отправке анкеты.
func TestСловарьСовпадаетСКлиентским(t *testing.T) {
	appDir := filepath.Join("..", "..", "app", "src", "data")

	index := readJS(t, filepath.Join(appDir, "index.js"))
	jsCompetencies := extractAll(t, `\{\s*code:\s*'([A-Z]+)'`, index)
	jsDestructors := extractAll(t, `\{\s*id:\s*'(d\d+)'`, index)

	assertSameSet(t, "компетенции", CompetencyCodes, jsCompetencies)
	assertSameSet(t, "деструкторы", DestructorCodes, jsDestructors)

	questionnaire := readJS(t, filepath.Join(appDir, "questionnaire.js"))
	jsRoles := extractAll(t, `\{\s*value:\s*'(subordinate|peer|manager|self)'`, questionnaire)
	assertSameSet(t, "роли", []string{
		string(RoleSubordinate), string(RolePeer), string(RoleManager), string(RoleSelf),
	}, jsRoles)

	jsTenures := extractAll(t, `\{\s*value:\s*'(lt3|3to12|gt12)'`, questionnaire)
	assertSameSet(t, "сроки наблюдения", []string{
		string(TenureLessThan3Months), string(Tenure3To12Months), string(TenureOver1Year),
	}, jsTenures)
}

func readJS(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("не прочитан %s: %v", path, err)
	}
	return string(body)
}

func extractAll(t *testing.T, pattern, body string) []string {
	t.Helper()
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		t.Fatalf("шаблон %q ничего не нашёл — изменилась форма файла, тест надо чинить", pattern)
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

func assertSameSet(t *testing.T, what string, goSide, jsSide []string) {
	t.Helper()
	a, b := append([]string(nil), goSide...), append([]string(nil), jsSide...)
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		t.Errorf("%s: в Go %d, в JS %d\n  Go: %v\n  JS: %v", what, len(a), len(b), a, b)
		return
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("%s расходятся:\n  Go: %v\n  JS: %v", what, a, b)
			return
		}
	}
}
