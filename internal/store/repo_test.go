package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mycontrolskill/internal/domain"
)

func newTestStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st, ctx
}

// seedAssessment заводит руководителя и раунд — почти каждому тесту ниже
// нужна эта пара.
func seedAssessment(t *testing.T, st *Store, ctx context.Context) (Leader, Assessment) {
	t.Helper()
	leader, err := st.EnsureLeader(ctx, "lead@example.com", "Руководитель")
	if err != nil {
		t.Fatalf("EnsureLeader: %v", err)
	}
	a, err := st.CreateAssessment(ctx, leader.ID, "Раунд 1")
	if err != nil {
		t.Fatalf("CreateAssessment: %v", err)
	}
	return leader, a
}

func score(v int) *int { return &v }

func fullSubmission(tenure domain.Tenure, value int) domain.Submission {
	sub := domain.Submission{Tenure: tenure}
	for _, code := range domain.CompetencyCodes {
		for i := 0; i < domain.ItemsPerCode; i++ {
			sub.Answers = append(sub.Answers, domain.Answer{
				Kind: domain.KindCompetency, Code: code, ItemIndex: i, Value: score(value),
			})
		}
	}
	for _, code := range domain.DestructorCodes {
		for i := 0; i < domain.ItemsPerCode; i++ {
			sub.Answers = append(sub.Answers, domain.Answer{
				Kind: domain.KindDestructor, Code: code, ItemIndex: i, Value: score(value),
			})
		}
	}
	return sub
}

func inviteAndSubmit(t *testing.T, st *Store, ctx context.Context, assessmentID string, role domain.Role, sub domain.Submission) Response {
	t.Helper()
	_, token, err := st.CreateInvite(ctx, assessmentID, role, "")
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	r, err := st.SubmitByToken(ctx, token, sub)
	if err != nil {
		t.Fatalf("SubmitByToken: %v", err)
	}
	return r
}

func TestEnsureLeaderИдемпотентенИНеЗависитОтРегистра(t *testing.T) {
	st, ctx := newTestStore(t)

	first, err := st.EnsureLeader(ctx, "Lead@Example.COM ", "Пётр")
	if err != nil {
		t.Fatalf("EnsureLeader: %v", err)
	}
	if first.Email != "lead@example.com" {
		t.Errorf("почта не нормализована: %q", first.Email)
	}

	second, err := st.EnsureLeader(ctx, "lead@example.com", "Другое имя")
	if err != nil {
		t.Fatalf("повторный EnsureLeader: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("создан второй аккаунт: %q и %q", first.ID, second.ID)
	}
	if second.Name != "Пётр" {
		t.Errorf("имя перезаписано на %q — вход не должен менять профиль", second.Name)
	}

	if _, err := st.EnsureLeader(ctx, "   ", ""); err == nil {
		t.Error("пустая почта должна отвергаться")
	}
}

func TestLeaderByIDНеНайден(t *testing.T) {
	st, ctx := newTestStore(t)

	if _, err := st.LeaderByID(ctx, "нет-такого"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ожидался ErrNotFound, получено: %v", err)
	}
}

func TestТокенПриглашенияВБазеНеХранится(t *testing.T) {
	st, ctx := newTestStore(t)
	_, a := seedAssessment(t, st, ctx)

	inv, token, err := st.CreateInvite(ctx, a.ID, domain.RolePeer, "peer@example.com")
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	var stored string
	if err := st.db.QueryRowContext(ctx,
		`SELECT token_hash FROM invite WHERE id = ?`, inv.ID).Scan(&stored); err != nil {
		t.Fatalf("чтение token_hash: %v", err)
	}
	if stored == token {
		t.Error("в базе лежит сам токен, а не его хэш")
	}
	if strings.Contains(stored, token) {
		t.Error("токен восстановим из сохранённого значения")
	}

	found, err := st.InviteByToken(ctx, token)
	if err != nil {
		t.Fatalf("InviteByToken: %v", err)
	}
	if found.ID != inv.ID {
		t.Errorf("найдено чужое приглашение: %q вместо %q", found.ID, inv.ID)
	}

	if _, err := st.InviteByToken(ctx, "подделанный-токен"); !errors.Is(err, ErrNotFound) {
		t.Errorf("по неверному токену ожидался ErrNotFound, получено: %v", err)
	}
}

func TestОтветНеСвязанСПриглашением(t *testing.T) {
	st, ctx := newTestStore(t)
	_, a := seedAssessment(t, st, ctx)

	inviteAndSubmit(t, st, ctx, a.ID, domain.RolePeer, fullSubmission(domain.TenureOver1Year, 4))

	// Анонимность 360° держится на том, что в response нет ни одной колонки,
	// ведущей к приглашению с почтой респондента.
	rows, err := st.db.QueryContext(ctx, `PRAGMA table_info(response)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid, notnull, pk int
			name, typ        string
			dflt             any
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("чтение колонок: %v", err)
		}
		if strings.Contains(name, "invite") || strings.Contains(name, "email") {
			t.Errorf("в response есть колонка %q — связь с респондентом восстановима", name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("чтение колонок: %v", err)
	}
}

func TestПовторнаяОтправкаПоСсылкеОтвергается(t *testing.T) {
	st, ctx := newTestStore(t)
	_, a := seedAssessment(t, st, ctx)

	_, token, err := st.CreateInvite(ctx, a.ID, domain.RolePeer, "")
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	if _, err := st.SubmitByToken(ctx, token, fullSubmission(domain.TenureOver1Year, 4)); err != nil {
		t.Fatalf("первая отправка: %v", err)
	}
	if _, err := st.SubmitByToken(ctx, token, fullSubmission(domain.TenureOver1Year, 5)); !errors.Is(err, ErrInviteUsed) {
		t.Errorf("вторая отправка: ожидался ErrInviteUsed, получено %v", err)
	}

	counts, err := st.CountResponses(ctx, a.ID)
	if err != nil {
		t.Fatalf("CountResponses: %v", err)
	}
	if counts.External != 1 {
		t.Errorf("сохранено анкет: %d, ожидалась одна", counts.External)
	}
}

func TestОдновременнаяОтправкаПоОднойСсылке(t *testing.T) {
	st, ctx := newTestStore(t)
	_, a := seedAssessment(t, st, ctx)

	_, token, err := st.CreateInvite(ctx, a.ID, domain.RolePeer, "")
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	// Двойной клик по кнопке — обычное дело. Пройти должна ровно одна
	// анкета: условие used_at IS NULL в UPDATE и есть та защита, на которую
	// это рассчитано.
	const attempts = 4
	var wg sync.WaitGroup
	errs := make([]error, attempts)
	start := make(chan struct{})
	for i := range attempts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = st.SubmitByToken(ctx, token, fullSubmission(domain.TenureOver1Year, 4))
		}(i)
	}
	close(start)
	wg.Wait()

	var ok, used int
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrInviteUsed):
			used++
		default:
			t.Errorf("неожиданная ошибка: %v", err)
		}
	}
	if ok != 1 {
		t.Errorf("успешных отправок = %d, ожидалась одна", ok)
	}
	if used != attempts-1 {
		t.Errorf("отказов ErrInviteUsed = %d, ожидалось %d", used, attempts-1)
	}

	counts, err := st.CountResponses(ctx, a.ID)
	if err != nil {
		t.Fatalf("CountResponses: %v", err)
	}
	if counts.External != 1 {
		t.Errorf("сохранено анкет: %d, ожидалась одна", counts.External)
	}
}

func TestРольБерётсяИзПриглашения(t *testing.T) {
	st, ctx := newTestStore(t)
	_, a := seedAssessment(t, st, ctx)

	_, token, err := st.CreateInvite(ctx, a.ID, domain.RoleSubordinate, "")
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	// Респондент прислал самооценку, хотя приглашён как подчинённый:
	// подмена роли изменила бы расчёт (самооценка в профиль не входит).
	sub := fullSubmission(domain.TenureOver1Year, 4)
	sub.Role = domain.RoleSelf

	r, err := st.SubmitByToken(ctx, token, sub)
	if err != nil {
		t.Fatalf("SubmitByToken: %v", err)
	}
	if r.Role != domain.RoleSubordinate {
		t.Errorf("роль = %q, ожидалась subordinate из приглашения", r.Role)
	}
}

func TestНекорректнаяАнкетаНеПогашаетПриглашение(t *testing.T) {
	st, ctx := newTestStore(t)
	_, a := seedAssessment(t, st, ctx)

	_, token, err := st.CreateInvite(ctx, a.ID, domain.RolePeer, "")
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	bad := domain.Submission{
		Tenure:  domain.TenureOver1Year,
		Answers: []domain.Answer{{Kind: domain.KindCompetency, Code: "НЕТ_ТАКОГО", ItemIndex: 0, Value: score(4)}},
	}
	if _, err := st.SubmitByToken(ctx, token, bad); err == nil {
		t.Fatal("анкета с неизвестным кодом принята")
	}

	// Транзакция откатилась — ссылка должна остаться рабочей, иначе
	// респондент потеряет доступ из-за ошибки клиента.
	inv, err := st.InviteByToken(ctx, token)
	if err != nil {
		t.Fatalf("InviteByToken: %v", err)
	}
	if inv.Used() {
		t.Error("приглашение погашено, хотя анкета не сохранена")
	}

	counts, err := st.CountResponses(ctx, a.ID)
	if err != nil {
		t.Fatalf("CountResponses: %v", err)
	}
	if counts.External != 0 {
		t.Errorf("сохранено анкет: %d, ожидалось ноль", counts.External)
	}
}

func TestЗакрытыйРаундНеПринимаетАнкеты(t *testing.T) {
	st, ctx := newTestStore(t)
	_, a := seedAssessment(t, st, ctx)

	_, token, err := st.CreateInvite(ctx, a.ID, domain.RolePeer, "")
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if err := st.CloseAssessment(ctx, a.ID); err != nil {
		t.Fatalf("CloseAssessment: %v", err)
	}

	if _, err := st.SubmitByToken(ctx, token, fullSubmission(domain.TenureOver1Year, 4)); !errors.Is(err, ErrAssessmentClosed) {
		t.Errorf("ожидался ErrAssessmentClosed, получено %v", err)
	}
	if _, _, err := st.CreateInvite(ctx, a.ID, domain.RolePeer, ""); !errors.Is(err, ErrAssessmentClosed) {
		t.Errorf("приглашение в закрытый раунд: ожидался ErrAssessmentClosed, получено %v", err)
	}

	// Повторное закрытие не должно падать и не должно сдвигать отметку.
	closed, err := st.AssessmentByID(ctx, a.ID)
	if err != nil {
		t.Fatalf("AssessmentByID: %v", err)
	}
	if err := st.CloseAssessment(ctx, a.ID); err != nil {
		t.Fatalf("повторный CloseAssessment: %v", err)
	}
	again, err := st.AssessmentByID(ctx, a.ID)
	if err != nil {
		t.Fatalf("AssessmentByID: %v", err)
	}
	if !again.ClosedAt.Equal(*closed.ClosedAt) {
		t.Error("повторное закрытие сдвинуло момент закрытия")
	}

	if err := st.CloseAssessment(ctx, "нет-такого"); !errors.Is(err, ErrNotFound) {
		t.Errorf("закрытие несуществующего раунда: ожидался ErrNotFound, получено %v", err)
	}
}

func TestCountResponsesРазделяетСамооценку(t *testing.T) {
	st, ctx := newTestStore(t)
	_, a := seedAssessment(t, st, ctx)

	inviteAndSubmit(t, st, ctx, a.ID, domain.RoleSelf, fullSubmission(domain.TenureOver1Year, 5))
	for _, role := range []domain.Role{domain.RolePeer, domain.RoleSubordinate} {
		inviteAndSubmit(t, st, ctx, a.ID, role, fullSubmission(domain.Tenure3To12Months, 4))
	}

	counts, err := st.CountResponses(ctx, a.ID)
	if err != nil {
		t.Fatalf("CountResponses: %v", err)
	}
	if counts.Self != 1 {
		t.Errorf("самооценок = %d, ожидалась одна", counts.Self)
	}
	if counts.External != 2 {
		t.Errorf("внешних = %d, ожидалось две", counts.External)
	}
	if counts.Ready() {
		t.Errorf("порог сработал на %d внешних, а нужно %d", counts.External, domain.MinRespondents)
	}

	inviteAndSubmit(t, st, ctx, a.ID, domain.RoleManager, fullSubmission(domain.TenureOver1Year, 3))
	counts, err = st.CountResponses(ctx, a.ID)
	if err != nil {
		t.Fatalf("CountResponses: %v", err)
	}
	if !counts.Ready() {
		t.Errorf("порог не сработал на %d внешних", counts.External)
	}
}

func TestResponsesForScoringГруппируетПоАнкетам(t *testing.T) {
	st, ctx := newTestStore(t)
	_, a := seedAssessment(t, st, ctx)

	// Полная анкета.
	inviteAndSubmit(t, st, ctx, a.ID, domain.RolePeer, fullSubmission(domain.TenureOver1Year, 4))

	// Анкета с «не могу оценить» и совсем пустая — обе должны доехать.
	partial := domain.Submission{
		Tenure: domain.Tenure3To12Months,
		Answers: []domain.Answer{
			{Kind: domain.KindCompetency, Code: "INT", ItemIndex: 0, Value: score(5)},
			{Kind: domain.KindCompetency, Code: "INT", ItemIndex: 1, Value: nil},
		},
	}
	inviteAndSubmit(t, st, ctx, a.ID, domain.RoleManager, partial)
	inviteAndSubmit(t, st, ctx, a.ID, domain.RoleSelf, domain.Submission{Tenure: domain.TenureLessThan3Months})

	got, err := st.ResponsesForScoring(ctx, a.ID)
	if err != nil {
		t.Fatalf("ResponsesForScoring: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("анкет = %d, ожидалось 3", len(got))
	}

	wantFull := (len(domain.CompetencyCodes) + len(domain.DestructorCodes)) * domain.ItemsPerCode
	if len(got[0].Answers) != wantFull {
		t.Errorf("оценок в полной анкете = %d, ожидалось %d", len(got[0].Answers), wantFull)
	}

	if len(got[1].Answers) != 2 {
		t.Fatalf("оценок в частичной анкете = %d, ожидалось 2", len(got[1].Answers))
	}
	var withValue, withoutValue int
	for _, ans := range got[1].Answers {
		if ans.Value == nil {
			withoutValue++
		} else {
			withValue++
		}
	}
	if withValue != 1 || withoutValue != 1 {
		t.Errorf("«не могу оценить» не отличается от оценки: с value %d, без %d", withValue, withoutValue)
	}

	// Пустая анкета не должна теряться: она есть в выборке, но без оценок.
	if got[2].Role != domain.RoleSelf {
		t.Errorf("третья анкета: роль %q, ожидалась self", got[2].Role)
	}
	if len(got[2].Answers) != 0 {
		t.Errorf("в пустой анкете %d оценок", len(got[2].Answers))
	}
}

func TestСостояниеРуководителя(t *testing.T) {
	st, ctx := newTestStore(t)
	leader, _ := seedAssessment(t, st, ctx)

	// Для нового аккаунта — пустой объект, а не ошибка.
	data, err := st.LoadLeaderState(ctx, leader.ID)
	if err != nil {
		t.Fatalf("LoadLeaderState: %v", err)
	}
	if string(data) != "{}" {
		t.Errorf("состояние нового аккаунта = %s", data)
	}

	want := json.RawMessage(`{"growthPoint":"COM","destructorAcknowledged":true}`)
	if err := st.SaveLeaderState(ctx, leader.ID, want); err != nil {
		t.Fatalf("SaveLeaderState: %v", err)
	}
	// Повторное сохранение должно перезаписывать, а не падать на конфликте.
	want = json.RawMessage(`{"growthPoint":"REL"}`)
	if err := st.SaveLeaderState(ctx, leader.ID, want); err != nil {
		t.Fatalf("повторный SaveLeaderState: %v", err)
	}

	got, err := st.LoadLeaderState(ctx, leader.ID)
	if err != nil {
		t.Fatalf("LoadLeaderState: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("состояние = %s, ожидалось %s", got, want)
	}

	if err := st.SaveLeaderState(ctx, leader.ID, json.RawMessage(`{битый`)); err == nil {
		t.Error("битый JSON принят")
	}
}

func TestРефлексииСвежиеПервыми(t *testing.T) {
	st, ctx := newTestStore(t)
	leader, _ := seedAssessment(t, st, ctx)

	for _, text := range []string{"первая", "вторая", "третья"} {
		if _, err := st.AddReflection(ctx, leader.ID, "COM", text); err != nil {
			t.Fatalf("AddReflection: %v", err)
		}
	}

	all, err := st.Reflections(ctx, leader.ID, 0)
	if err != nil {
		t.Fatalf("Reflections: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("записей = %d, ожидалось 3", len(all))
	}

	limited, err := st.Reflections(ctx, leader.ID, 2)
	if err != nil {
		t.Fatalf("Reflections с limit: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("записей с limit=2: %d", len(limited))
	}
	if limited[0].Text != all[0].Text {
		t.Errorf("порядок при limit отличается: %q и %q", limited[0].Text, all[0].Text)
	}
}

func TestУдалениеРуководителяЧиститВсёЕгоДерево(t *testing.T) {
	st, ctx := newTestStore(t)
	leader, a := seedAssessment(t, st, ctx)

	inviteAndSubmit(t, st, ctx, a.ID, domain.RolePeer, fullSubmission(domain.TenureOver1Year, 4))
	if _, err := st.AddReflection(ctx, leader.ID, "COM", "запись"); err != nil {
		t.Fatalf("AddReflection: %v", err)
	}

	if _, err := st.db.ExecContext(ctx, `DELETE FROM leader WHERE id = ?`, leader.ID); err != nil {
		t.Fatalf("удаление руководителя: %v", err)
	}

	// Каскад проверяем по самой дальней таблице: если сработал он, то
	// сработали и промежуточные.
	for _, table := range []string{"assessment", "invite", "response", "answer", "reflection"} {
		var n int
		if err := st.db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			t.Fatalf("подсчёт в %s: %v", table, err)
		}
		if n != 0 {
			t.Errorf("в таблице %s осталось %d строк после удаления руководителя", table, n)
		}
	}
}

func TestAssessmentsByLeaderСвежиеПервыми(t *testing.T) {
	st, ctx := newTestStore(t)
	leader, first := seedAssessment(t, st, ctx)

	second, err := st.CreateAssessment(ctx, leader.ID, "Раунд 2")
	if err != nil {
		t.Fatalf("CreateAssessment: %v", err)
	}

	list, err := st.AssessmentsByLeader(ctx, leader.ID)
	if err != nil {
		t.Fatalf("AssessmentsByLeader: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("раундов = %d, ожидалось 2", len(list))
	}
	if list[0].ID != second.ID || list[1].ID != first.ID {
		t.Errorf("порядок раундов неверный: %q, %q", list[0].Title, list[1].Title)
	}

	if _, err := st.AssessmentByID(ctx, "нет-такого"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ожидался ErrNotFound, получено %v", err)
	}
}
