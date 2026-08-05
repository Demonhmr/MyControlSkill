package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"mycontrolskill/internal/domain"
)

// countRows считает строки во всех таблицах с данными.
func countRows(t *testing.T, st *Store, ctx context.Context) map[string]int {
	t.Helper()
	tables := []string{
		"leader", "assessment", "invite", "response", "answer", "open_answer",
		"leader_state", "reflection", "session", "login_token", "org", "org_member",
		"consent_event",
	}
	out := map[string]int{}
	for _, table := range tables {
		var n int
		if err := st.db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&n); err != nil {
			t.Fatalf("подсчёт в %s: %v", table, err)
		}
		out[table] = n
	}
	return out
}

// seedFullLeader создаёт руководителя со всеми видами данных.
func seedFullLeader(t *testing.T, st *Store, ctx context.Context, email string) Leader {
	t.Helper()

	leader, err := st.EnsureLeader(ctx, email, "")
	if err != nil {
		t.Fatalf("EnsureLeader: %v", err)
	}
	assessment, err := st.CreateAssessment(ctx, leader.ID, "Раунд")
	if err != nil {
		t.Fatalf("CreateAssessment: %v", err)
	}

	sub := fullSubmission(domain.TenureOver1Year, 4)
	sub.OpenAnswers = []domain.OpenAnswer{{QuestionIndex: 0, Text: "Комментарий"}}
	inviteAndSubmit(t, st, ctx, assessment.ID, domain.RolePeer, sub)

	if _, _, err := st.CreateInvite(ctx, assessment.ID, domain.RoleManager, "m@example.com"); err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if _, _, err := st.CreateSession(ctx, leader.ID); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := st.CreateLoginToken(ctx, email); err != nil {
		t.Fatalf("CreateLoginToken: %v", err)
	}
	if err := st.SaveLeaderState(ctx, leader.ID, json.RawMessage(`{"growthPoint":"COM"}`)); err != nil {
		t.Fatalf("SaveLeaderState: %v", err)
	}
	if _, err := st.AddReflection(ctx, leader.ID, "COM", "Практика"); err != nil {
		t.Fatalf("AddReflection: %v", err)
	}
	return leader
}

func TestУдалениеАккаунтаЧиститВсёДерево(t *testing.T) {
	st, ctx := newTestStore(t)
	leader := seedFullLeader(t, st, ctx, "lead@example.com")

	before := countRows(t, st, ctx)
	for _, table := range []string{"assessment", "response", "answer", "open_answer", "invite", "session", "login_token", "leader_state", "reflection"} {
		if before[table] == 0 {
			t.Fatalf("тест бесполезен: таблица %s пуста до удаления", table)
		}
	}

	if err := st.DeleteLeader(ctx, leader.ID); err != nil {
		t.Fatalf("DeleteLeader: %v", err)
	}

	after := countRows(t, st, ctx)
	for table, n := range after {
		if n != 0 {
			t.Errorf("в таблице %s осталось %d строк", table, n)
		}
	}
}

// Ссылки для входа привязаны к почте, а не к аккаунту: каскад их не заденет,
// и невыбранные ссылки пережили бы удаление.
func TestУдалениеЧиститСсылкиДляВходаПоПочте(t *testing.T) {
	st, ctx := newTestStore(t)
	leader := seedFullLeader(t, st, ctx, "lead@example.com")

	token, err := st.CreateLoginToken(ctx, "lead@example.com")
	if err != nil {
		t.Fatalf("CreateLoginToken: %v", err)
	}
	if err := st.DeleteLeader(ctx, leader.ID); err != nil {
		t.Fatalf("DeleteLeader: %v", err)
	}

	if _, err := st.ConsumeLoginToken(ctx, token, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("ссылка пережила удаление аккаунта: %v", err)
	}
}

func TestУдалениеНеТрогаетЧужиеДанные(t *testing.T) {
	st, ctx := newTestStore(t)
	victim := seedFullLeader(t, st, ctx, "victim@example.com")
	other := seedFullLeader(t, st, ctx, "other@example.com")

	if err := st.DeleteLeader(ctx, victim.ID); err != nil {
		t.Fatalf("DeleteLeader: %v", err)
	}

	if _, err := st.LeaderByID(ctx, other.ID); err != nil {
		t.Fatalf("чужой аккаунт задет: %v", err)
	}
	list, err := st.AssessmentsByLeader(ctx, other.ID)
	if err != nil {
		t.Fatalf("AssessmentsByLeader: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("у чужого аккаунта раундов %d, ожидался один", len(list))
	}
}

func TestУдалениеНесуществующегоАккаунта(t *testing.T) {
	st, ctx := newTestStore(t)

	if err := st.DeleteLeader(ctx, "нет-такого"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ожидался ErrNotFound, получено %v", err)
	}
}

// Организация без единственного эйчара становится никому не видна, поэтому
// уходит вместе с ним. Чужие аккаунты при этом остаются.
func TestУдалениеЭйчараУноситОсиротевшуюОрганизацию(t *testing.T) {
	st, ctx := newTestStore(t)

	hr, _ := st.EnsureLeader(ctx, "hr@example.com", "")
	org, err := st.CreateOrg(ctx, "Компас", hr.ID)
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}
	member, err := st.AddOrgMember(ctx, org.ID, "lead@example.com", OrgRoleLeader)
	if err != nil {
		t.Fatalf("AddOrgMember: %v", err)
	}

	if err := st.DeleteLeader(ctx, hr.ID); err != nil {
		t.Fatalf("DeleteLeader: %v", err)
	}

	if _, err := st.LeaderByID(ctx, member.LeaderID); err != nil {
		t.Errorf("участник удалён вместе с организацией: %v", err)
	}
	if _, _, err := st.OrgForLeader(ctx, member.LeaderID); !errors.Is(err, ErrNotFound) {
		t.Errorf("участие в удалённой организации осталось: %v", err)
	}
}

func TestОрганизацияСДругимЭйчаромВыживает(t *testing.T) {
	st, ctx := newTestStore(t)

	first, _ := st.EnsureLeader(ctx, "hr1@example.com", "")
	org, _ := st.CreateOrg(ctx, "Компас", first.ID)
	second, err := st.AddOrgMember(ctx, org.ID, "hr2@example.com", OrgRoleHR)
	if err != nil {
		t.Fatalf("AddOrgMember: %v", err)
	}

	if err := st.DeleteLeader(ctx, first.ID); err != nil {
		t.Fatalf("DeleteLeader: %v", err)
	}

	got, role, err := st.OrgForLeader(ctx, second.LeaderID)
	if err != nil {
		t.Fatalf("организация удалена при живом втором эйчаре: %v", err)
	}
	if got.ID != org.ID || role != OrgRoleHR {
		t.Errorf("участие второго эйчара изменилось: %q, %q", got.ID, role)
	}
}

func TestУдалениеРаунда(t *testing.T) {
	st, ctx := newTestStore(t)
	leader, assessment := seedAssessment(t, st, ctx)
	inviteAndSubmit(t, st, ctx, assessment.ID, domain.RolePeer, fullSubmission(domain.TenureOver1Year, 4))

	if err := st.DeleteAssessment(ctx, assessment.ID); err != nil {
		t.Fatalf("DeleteAssessment: %v", err)
	}

	if _, err := st.AssessmentByID(ctx, assessment.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("раунд не удалён: %v", err)
	}
	// Аккаунт при этом остаётся: удаление раунда — не удаление человека.
	if _, err := st.LeaderByID(ctx, leader.ID); err != nil {
		t.Errorf("аккаунт удалён вместе с раундом: %v", err)
	}

	counts := countRows(t, st, ctx)
	for _, table := range []string{"response", "answer", "invite"} {
		if counts[table] != 0 {
			t.Errorf("в таблице %s осталось %d строк", table, counts[table])
		}
	}

	if err := st.DeleteAssessment(ctx, assessment.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("повторное удаление: ожидался ErrNotFound, получено %v", err)
	}
}

func TestЧисткаПоСрокуХранения(t *testing.T) {
	st, ctx := newTestStore(t)
	leader, fresh := seedAssessment(t, st, ctx)

	old, err := st.CreateAssessment(ctx, leader.ID, "Прошлогодний")
	if err != nil {
		t.Fatalf("CreateAssessment: %v", err)
	}
	// Двигаем дату напрямую: ждать год в тесте нечем.
	if _, err := st.db.ExecContext(ctx,
		`UPDATE assessment SET created_at = ? WHERE id = ?`,
		time.Now().AddDate(-2, 0, 0).UTC().Format(timeLayout), old.ID); err != nil {
		t.Fatalf("сдвиг даты: %v", err)
	}

	n, err := st.PurgeOlderThan(ctx, time.Now().AddDate(-1, 0, 0))
	if err != nil {
		t.Fatalf("PurgeOlderThan: %v", err)
	}
	if n != 1 {
		t.Errorf("удалено раундов %d, ожидался один", n)
	}

	if _, err := st.AssessmentByID(ctx, old.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("старый раунд остался: %v", err)
	}
	// Свежий раунд и сам аккаунт не трогаются: стареет обратная связь, а не
	// личный кабинет.
	if _, err := st.AssessmentByID(ctx, fresh.ID); err != nil {
		t.Errorf("свежий раунд удалён: %v", err)
	}
	if _, err := st.LeaderByID(ctx, leader.ID); err != nil {
		t.Errorf("аккаунт удалён чисткой: %v", err)
	}
}
