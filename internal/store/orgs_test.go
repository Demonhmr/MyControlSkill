package store

import (
	"errors"
	"testing"
)

func TestСоздательОрганизацииСтановитсяЭйчаром(t *testing.T) {
	st, ctx := newTestStore(t)

	hr, err := st.EnsureLeader(ctx, "hr@example.com", "")
	if err != nil {
		t.Fatalf("EnsureLeader: %v", err)
	}

	org, err := st.CreateOrg(ctx, "  ООО «Компас»  ", hr.ID)
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}
	if org.Name != "ООО «Компас»" {
		t.Errorf("название не обрезано по краям: %q", org.Name)
	}

	got, role, err := st.OrgForLeader(ctx, hr.ID)
	if err != nil {
		t.Fatalf("OrgForLeader: %v", err)
	}
	if got.ID != org.ID {
		t.Errorf("организация = %q, ожидалась %q", got.ID, org.ID)
	}
	// Организация без единого эйчара никому не видна, поэтому создатель им
	// и становится.
	if role != OrgRoleHR {
		t.Errorf("роль создателя = %q, ожидалась hr", role)
	}
}

func TestБезНазванияОрганизацияНеСоздаётся(t *testing.T) {
	st, ctx := newTestStore(t)
	hr, _ := st.EnsureLeader(ctx, "hr@example.com", "")

	if _, err := st.CreateOrg(ctx, "   ", hr.ID); err == nil {
		t.Error("организация без названия создана")
	}
}

func TestВтораяОрганизацияНеСоздаётся(t *testing.T) {
	st, ctx := newTestStore(t)
	hr, _ := st.EnsureLeader(ctx, "hr@example.com", "")

	if _, err := st.CreateOrg(ctx, "Первая", hr.ID); err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}
	// Одна организация на человека: иначе непонятно, чью сводку показывать.
	if _, err := st.CreateOrg(ctx, "Вторая", hr.ID); !errors.Is(err, ErrAlreadyInOrg) {
		t.Errorf("ожидался ErrAlreadyInOrg, получено %v", err)
	}
}

func TestУчастникДобавляетсяПоПочтеИАккаунтЗаводится(t *testing.T) {
	st, ctx := newTestStore(t)
	hr, _ := st.EnsureLeader(ctx, "hr@example.com", "")
	org, _ := st.CreateOrg(ctx, "Компас", hr.ID)

	// Аккаунта ещё нет: эйчар собирает состав до первого входа людей.
	if _, err := st.LeaderByEmail(ctx, "lead@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("аккаунт не должен существовать заранее: %v", err)
	}

	member, err := st.AddOrgMember(ctx, org.ID, " Lead@Example.COM ", OrgRoleLeader)
	if err != nil {
		t.Fatalf("AddOrgMember: %v", err)
	}
	if member.Email != "lead@example.com" {
		t.Errorf("почта не нормализована: %q", member.Email)
	}

	// Первый вход подхватит уже готовый аккаунт, а не заведёт второй.
	token, err := st.CreateLoginToken(ctx, "lead@example.com")
	if err != nil {
		t.Fatalf("CreateLoginToken: %v", err)
	}
	logged, err := st.ConsumeLoginToken(ctx, token, nil)
	if err != nil {
		t.Fatalf("ConsumeLoginToken: %v", err)
	}
	if logged.ID != member.LeaderID {
		t.Errorf("вход завёл второй аккаунт: %q и %q", logged.ID, member.LeaderID)
	}
}

func TestПовторноеДобавлениеНеОшибка(t *testing.T) {
	st, ctx := newTestStore(t)
	hr, _ := st.EnsureLeader(ctx, "hr@example.com", "")
	org, _ := st.CreateOrg(ctx, "Компас", hr.ID)

	first, err := st.AddOrgMember(ctx, org.ID, "lead@example.com", OrgRoleLeader)
	if err != nil {
		t.Fatalf("AddOrgMember: %v", err)
	}
	// Эйчар мог просто нажать дважды — состав от этого не меняется.
	again, err := st.AddOrgMember(ctx, org.ID, "lead@example.com", OrgRoleLeader)
	if err != nil {
		t.Fatalf("повторный AddOrgMember: %v", err)
	}
	if again.LeaderID != first.LeaderID {
		t.Error("повторное добавление завело второго участника")
	}

	members, err := st.OrgMembers(ctx, org.ID)
	if err != nil {
		t.Fatalf("OrgMembers: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("участников %d, ожидалось двое (эйчар и руководитель)", len(members))
	}
}

func TestЧужогоУчастникаНеПереманить(t *testing.T) {
	st, ctx := newTestStore(t)

	firstHR, _ := st.EnsureLeader(ctx, "hr1@example.com", "")
	firstOrg, _ := st.CreateOrg(ctx, "Первая", firstHR.ID)
	if _, err := st.AddOrgMember(ctx, firstOrg.ID, "lead@example.com", OrgRoleLeader); err != nil {
		t.Fatalf("AddOrgMember: %v", err)
	}

	secondHR, _ := st.EnsureLeader(ctx, "hr2@example.com", "")
	secondOrg, _ := st.CreateOrg(ctx, "Вторая", secondHR.ID)

	// Иначе чужой эйчар получал бы доступ к сводке по человеку, просто
	// добавив его к себе.
	if _, err := st.AddOrgMember(ctx, secondOrg.ID, "lead@example.com", OrgRoleLeader); !errors.Is(err, ErrAlreadyInOrg) {
		t.Errorf("ожидался ErrAlreadyInOrg, получено %v", err)
	}
}

func TestБезОрганизацииОтдаётсяErrNotFound(t *testing.T) {
	st, ctx := newTestStore(t)
	lead, _ := st.EnsureLeader(ctx, "lead@example.com", "")

	if _, _, err := st.OrgForLeader(ctx, lead.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("ожидался ErrNotFound, получено %v", err)
	}
}

func TestLatestAssessmentБерётСвежий(t *testing.T) {
	st, ctx := newTestStore(t)
	leader, first := seedAssessment(t, st, ctx)

	second, err := st.CreateAssessment(ctx, leader.ID, "Раунд 2")
	if err != nil {
		t.Fatalf("CreateAssessment: %v", err)
	}

	got, err := st.LatestAssessment(ctx, leader.ID)
	if err != nil {
		t.Fatalf("LatestAssessment: %v", err)
	}
	if got.ID != second.ID {
		t.Errorf("взят раунд %q, ожидался свежий %q (первый — %q)", got.ID, second.ID, first.ID)
	}

	other, _ := st.EnsureLeader(ctx, "other@example.com", "")
	if _, err := st.LatestAssessment(ctx, other.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("для руководителя без раундов ожидался ErrNotFound, получено %v", err)
	}
}

func TestСогласияПоУмолчаниюНет(t *testing.T) {
	st, ctx := newTestStore(t)
	hr, _ := st.EnsureLeader(ctx, "hr@example.com", "")
	org, _ := st.CreateOrg(ctx, "Компас", hr.ID)

	member, err := st.AddOrgMember(ctx, org.ID, "lead@example.com", OrgRoleLeader)
	if err != nil {
		t.Fatalf("AddOrgMember: %v", err)
	}
	// Молчание согласием не считается: добавление в организацию — решение
	// эйчара, а не человека.
	if member.ProfileConsentGranted() {
		t.Error("новый участник считается давшим согласие")
	}
}

func TestСогласиеВыдаётсяИОтзывается(t *testing.T) {
	st, ctx := newTestStore(t)
	hr, _ := st.EnsureLeader(ctx, "hr@example.com", "")
	org, _ := st.CreateOrg(ctx, "Компас", hr.ID)
	added, _ := st.AddOrgMember(ctx, org.ID, "lead@example.com", OrgRoleLeader)

	granted, err := st.SetProfileConsent(ctx, added.LeaderID, true)
	if err != nil {
		t.Fatalf("SetProfileConsent: %v", err)
	}
	if !granted.ProfileConsentGranted() {
		t.Fatal("согласие не выдано")
	}

	withdrawn, err := st.SetProfileConsent(ctx, added.LeaderID, false)
	if err != nil {
		t.Fatalf("отзыв согласия: %v", err)
	}
	if withdrawn.ProfileConsentGranted() {
		t.Error("согласие не отозвано")
	}

	// Журнал хранит оба события: отозванное согласие иначе неотличимо от
	// никогда не выданного.
	history, err := st.ConsentHistory(ctx, added.LeaderID)
	if err != nil {
		t.Fatalf("ConsentHistory: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("записей в журнале %d, ожидалось две", len(history))
	}
	if history[0].Granted || !history[1].Granted {
		t.Errorf("порядок событий неверный: %+v", history)
	}
}

func TestПовторнаяВыдачаНеСдвигаетМомент(t *testing.T) {
	st, ctx := newTestStore(t)
	hr, _ := st.EnsureLeader(ctx, "hr@example.com", "")
	org, _ := st.CreateOrg(ctx, "Компас", hr.ID)
	added, _ := st.AddOrgMember(ctx, org.ID, "lead@example.com", OrgRoleLeader)

	first, _ := st.SetProfileConsent(ctx, added.LeaderID, true)
	again, err := st.SetProfileConsent(ctx, added.LeaderID, true)
	if err != nil {
		t.Fatalf("повторная выдача: %v", err)
	}

	// Человек согласился тогда, когда согласился, а не когда последний раз
	// нажал кнопку.
	if !again.ProfileConsentAt.Equal(*first.ProfileConsentAt) {
		t.Error("повторная выдача сдвинула момент согласия")
	}
	history, _ := st.ConsentHistory(ctx, added.LeaderID)
	if len(history) != 1 {
		t.Errorf("записей в журнале %d, ожидалась одна", len(history))
	}
}

func TestСогласиеБезОрганизацииНевозможно(t *testing.T) {
	st, ctx := newTestStore(t)
	lone, _ := st.EnsureLeader(ctx, "lone@example.com", "")

	if _, err := st.SetProfileConsent(ctx, lone.ID, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("ожидался ErrNotFound, получено %v", err)
	}
}
