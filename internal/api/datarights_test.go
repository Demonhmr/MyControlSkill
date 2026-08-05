package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"mycontrolskill/internal/domain"
	"mycontrolskill/internal/store"
)

func TestВыгрузкаСодержитСвоиДанные(t *testing.T) {
	ts, mailer, st := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")

	id := createAssessment(t, client, ts, "Пилот")
	doJSON(t, client, http.MethodPost, ts.URL+"/api/assessments/"+id+"/invites",
		`{"role":"peer","email":"peer@example.com"}`)
	submitResponses(t, st, id,
		[]domain.Role{domain.RolePeer, domain.RoleSubordinate, domain.RoleManager}, 5)
	doJSON(t, client, http.MethodPut, ts.URL+"/api/state", `{"state":{"growthPoint":"COM"}}`)
	doJSON(t, client, http.MethodPost, ts.URL+"/api/reflections", `{"code":"COM","text":"Практика"}`)

	resp, body := doJSON(t, client, http.MethodGet, ts.URL+"/api/me/export", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("статус %d, ожидался 200", resp.StatusCode)
	}
	// Выгрузку сохраняют файлом, а не читают вкладкой.
	if cd := resp.Header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment;") {
		t.Errorf("Content-Disposition = %q", cd)
	}

	account, _ := body["account"].(map[string]any)
	if account["email"] != "lead@example.com" {
		t.Errorf("почта в выгрузке = %v", account["email"])
	}

	assessments, _ := body["assessments"].([]any)
	if len(assessments) != 1 {
		t.Fatalf("раундов в выгрузке %d, ожидался один", len(assessments))
	}
	first, _ := assessments[0].(map[string]any)
	if first["title"] != "Пилот" {
		t.Errorf("название раунда = %v", first["title"])
	}
	if invites, _ := first["invites"].([]any); len(invites) == 0 {
		t.Error("приглашения не попали в выгрузку")
	}
	// Профиль посчитанного раунда — это то, что руководитель и так видит.
	if first["profile"] == nil {
		t.Error("профиль посчитанного раунда не попал в выгрузку")
	}

	state, _ := body["state"].(map[string]any)
	if state["growthPoint"] != "COM" {
		t.Errorf("состояние в выгрузке = %v", state)
	}
	if refs, _ := body["reflections"].([]any); len(refs) != 1 {
		t.Errorf("записей тренажёра в выгрузке %d, ожидалась одна", len(refs))
	}
}

// Право на свои данные не распространяется на чужие: по отдельным ответам
// восстанавливается, кто именно как ответил.
func TestВыгрузкаНеСодержитСырыхОтветов(t *testing.T) {
	ts, mailer, st := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")

	id := createAssessment(t, client, ts, "Пилот")
	submitResponses(t, st, id,
		[]domain.Role{domain.RolePeer, domain.RoleSubordinate, domain.RoleManager}, 5)

	_, body := doJSON(t, client, http.MethodGet, ts.URL+"/api/me/export", "")
	raw, _ := json.Marshal(body)
	for _, leak := range []string{"answers", "itemIndex", "tenure", "openAnswers", "responses"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("в выгрузке есть %q — это сырые данные анкет", leak)
		}
	}
	// И об этом сказано в самом файле: получатель не должен гадать, почему
	// отдельных ответов нет.
	if note, _ := body["_note"].(string); !strings.Contains(note, "анонимность") {
		t.Errorf("в выгрузке нет пояснения: %q", note)
	}
}

func TestУдалениеАккаунтаТребуетПодтверждения(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")

	cases := map[string]string{
		"без подтверждения": `{}`,
		"чужой адрес":       `{"confirmEmail":"other@example.com"}`,
		"пустая строка":     `{"confirmEmail":"   "}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			resp, _ := doJSON(t, client, http.MethodDelete, ts.URL+"/api/me", payload)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("статус %d, ожидался 400", resp.StatusCode)
			}
		})
	}

	// Аккаунт цел.
	resp, _ := doJSON(t, client, http.MethodGet, ts.URL+"/api/me", "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("аккаунт задет неудачной попыткой: статус %d", resp.StatusCode)
	}
}

func TestУдалениеАккаунтаУноситВсё(t *testing.T) {
	ts, mailer, st := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")

	id := createAssessment(t, client, ts, "Пилот")
	submitResponses(t, st, id, []domain.Role{domain.RolePeer}, 5)
	doJSON(t, client, http.MethodPut, ts.URL+"/api/state", `{"state":{"growthPoint":"COM"}}`)

	resp, _ := doJSON(t, client, http.MethodDelete, ts.URL+"/api/me",
		`{"confirmEmail":"Lead@Example.com"}`)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("статус %d, ожидался 204", resp.StatusCode)
	}

	// Сессия недействительна, аккаунта нет.
	me, _ := doJSON(t, client, http.MethodGet, ts.URL+"/api/me", "")
	if me.StatusCode != http.StatusUnauthorized {
		t.Errorf("после удаления /api/me вернул %d", me.StatusCode)
	}
	if _, err := st.LeaderByEmail(t.Context(), "lead@example.com"); err == nil {
		t.Error("аккаунт остался в базе")
	}

	// Данные раунда ушли вместе с ним.
	if _, err := st.AssessmentByID(t.Context(), id); err == nil {
		t.Error("раунд пережил удаление аккаунта")
	}
}

func TestУдалениеРаундаЧерезHTTP(t *testing.T) {
	ts, mailer, st := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")

	id := createAssessment(t, client, ts, "Пилот")
	submitResponses(t, st, id, []domain.Role{domain.RolePeer}, 5)

	resp, _ := doJSON(t, client, http.MethodDelete, ts.URL+"/api/assessments/"+id, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("статус %d, ожидался 204", resp.StatusCode)
	}

	// Аккаунт остаётся: удаление раунда — не удаление человека.
	me, _ := doJSON(t, client, http.MethodGet, ts.URL+"/api/me", "")
	if me.StatusCode != http.StatusOK {
		t.Errorf("аккаунт задет удалением раунда: статус %d", me.StatusCode)
	}
	if _, err := st.AssessmentByID(t.Context(), id); err == nil {
		t.Error("раунд не удалён")
	}

	resp, _ = doJSON(t, client, http.MethodDelete, ts.URL+"/api/assessments/"+id, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("повторное удаление: статус %d, ожидался 404", resp.StatusCode)
	}
}

func TestЧужойРаундНеУдалить(t *testing.T) {
	ts, mailer, st := newTestServer(t)

	owner := login(t, ts, mailer, "owner@example.com")
	id := createAssessment(t, owner, ts, "Чужой")

	stranger := login(t, ts, mailer, "stranger@example.com")
	resp, _ := doJSON(t, stranger, http.MethodDelete, ts.URL+"/api/assessments/"+id, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("статус %d, ожидался 404", resp.StatusCode)
	}

	if _, err := st.AssessmentByID(t.Context(), id); err != nil {
		t.Errorf("чужой раунд удалён: %v", err)
	}
}

func TestПраваНаДанныеТребуютВхода(t *testing.T) {
	ts, _, _ := newTestServer(t)
	client := newClient(t)

	paths := []struct{ method, path, body string }{
		{http.MethodGet, "/api/me/export", ""},
		{http.MethodDelete, "/api/me", `{"confirmEmail":"x@example.com"}`},
		{http.MethodDelete, "/api/assessments/любой", ""},
	}
	for _, p := range paths {
		resp, _ := doJSON(t, client, p.method, ts.URL+p.path, p.body)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: статус %d, ожидался 401", p.method, p.path, resp.StatusCode)
		}
	}
	_ = store.ErrNotFound
}
