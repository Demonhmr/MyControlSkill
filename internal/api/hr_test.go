package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mycontrolskill/internal/domain"
	"mycontrolskill/internal/store"
)

func createOrg(t *testing.T, client *http.Client, ts *httptest.Server, name string) string {
	t.Helper()
	resp, body := doJSON(t, client, http.MethodPost, ts.URL+"/api/hr/org", `{"name":`+quote(name)+`}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("создание организации вернуло %d: %v", resp.StatusCode, body)
	}
	id, _ := body["id"].(string)
	return id
}

func addMember(t *testing.T, client *http.Client, ts *httptest.Server, email string) map[string]any {
	t.Helper()
	resp, body := doJSON(t, client, http.MethodPost, ts.URL+"/api/hr/members", `{"email":`+quote(email)+`}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("добавление участника вернуло %d: %v", resp.StatusCode, body)
	}
	return body
}

// grantConsent входит под участником и разрешает показ профиля HR.
// Без этого сводка чисел не отдаёт, сколько бы анкет ни собралось.
func grantConsent(t *testing.T, ts *httptest.Server, mailer *recordingMailer, email string) {
	t.Helper()
	client := login(t, ts, mailer, email)
	resp, body := doJSON(t, client, http.MethodPut, ts.URL+"/api/me/org/consent", `{"granted":true}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("выдача согласия вернула %d: %v", resp.StatusCode, body)
	}
}

func overview(t *testing.T, client *http.Client, ts *httptest.Server) map[string]any {
	t.Helper()
	resp, body := doJSON(t, client, http.MethodGet, ts.URL+"/api/hr/overview", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("сводка вернула %d: %v", resp.StatusCode, body)
	}
	return body
}

// leaderRow находит строку сводки по адресу почты.
func leaderRow(t *testing.T, body map[string]any, email string) map[string]any {
	t.Helper()
	rows, _ := body["leaders"].([]any)
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		if row["email"] == email {
			return row
		}
	}
	t.Fatalf("в сводке нет строки для %s", email)
	return nil
}

func TestСводкаПоказываетСоставИПричиныПустоты(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	hr := login(t, ts, mailer, "hr@example.com")
	createOrg(t, hr, ts, "ООО «Компас»")
	addMember(t, hr, ts, "lead@example.com")
	grantConsent(t, ts, mailer, "lead@example.com")

	body := overview(t, hr, ts)
	org, _ := body["org"].(map[string]any)
	if org["name"] != "ООО «Компас»" {
		t.Errorf("название организации = %v", org["name"])
	}

	rows, _ := body["leaders"].([]any)
	if len(rows) != 2 {
		t.Fatalf("строк в сводке %d, ожидалось две (эйчар и руководитель)", len(rows))
	}

	// У руководителя нет ни одного раунда: показываем это прямо, а не
	// нулевыми перцентилями.
	row := leaderRow(t, body, "lead@example.com")
	if row["ready"] != false {
		t.Errorf("строка без раундов помечена готовой")
	}
	counts, _ := row["counts"].(map[string]any)
	if counts["required"] != float64(domain.MinRespondents) {
		t.Errorf("порог в строке = %v", counts["required"])
	}
	if d, _ := row["destructors"].([]any); len(d) != 0 {
		t.Errorf("у неготовой строки есть деструкторы: %v", d)
	}
}

func TestСводкаНеПоказываетЧиселНижеПорога(t *testing.T) {
	ts, mailer, st := newTestServer(t)
	hr := login(t, ts, mailer, "hr@example.com")
	createOrg(t, hr, ts, "Компас")
	member := addMember(t, hr, ts, "lead@example.com")

	// Два ответа — меньше порога: организационные решения по такому набору
	// принимались бы по шуму.
	leaderID, _ := member["leaderId"].(string)
	assessment, err := st.CreateAssessment(t.Context(), leaderID, "Раунд")
	if err != nil {
		t.Fatalf("CreateAssessment: %v", err)
	}
	submitResponses(t, st, assessment.ID, []domain.Role{domain.RolePeer, domain.RoleSubordinate}, 5)
	grantConsent(t, ts, mailer, "lead@example.com")

	body := overview(t, hr, ts)
	row := leaderRow(t, body, "lead@example.com")
	if row["ready"] != false {
		t.Error("строка помечена готовой ниже порога")
	}
	counts, _ := row["counts"].(map[string]any)
	if counts["external"] != float64(2) {
		t.Errorf("внешних анкет = %v, ожидалось 2", counts["external"])
	}

	raw, _ := json.Marshal(row)
	if strings.Contains(string(raw), `"percentile"`) {
		t.Errorf("ниже порога отдан перцентиль: %s", raw)
	}
}

func TestСводкаСчитаетДеструкторыИСильныеСтороны(t *testing.T) {
	ts, mailer, st := newTestServer(t)
	hr := login(t, ts, mailer, "hr@example.com")
	createOrg(t, hr, ts, "Компас")
	member := addMember(t, hr, ts, "lead@example.com")

	leaderID, _ := member["leaderId"].(string)
	assessment, err := st.CreateAssessment(t.Context(), leaderID, "Раунд")
	if err != nil {
		t.Fatalf("CreateAssessment: %v", err)
	}
	submitResponses(t, st, assessment.ID,
		[]domain.Role{domain.RolePeer, domain.RoleSubordinate, domain.RoleManager}, 5)
	grantConsent(t, ts, mailer, "lead@example.com")

	body := overview(t, hr, ts)
	row := leaderRow(t, body, "lead@example.com")
	if row["ready"] != true {
		t.Fatalf("строка не помечена готовой: %v", row)
	}

	destructors, _ := row["destructors"].([]any)
	if len(destructors) != len(domain.DestructorCodes) {
		t.Errorf("деструкторов в строке %d, ожидалось %d", len(destructors), len(domain.DestructorCodes))
	}

	// Сильных сторон показывается не больше двух: сводка про приоритеты,
	// а не про полный профиль.
	strengths, _ := row["strengths"].([]any)
	if len(strengths) > TopStrengths {
		t.Errorf("сильных сторон %d, предел %d", len(strengths), TopStrengths)
	}
	if len(strengths) == 2 {
		first, _ := strengths[0].(map[string]any)
		second, _ := strengths[1].(map[string]any)
		if first["percentile"].(float64) < second["percentile"].(float64) {
			t.Error("сильные стороны не отсортированы по убыванию")
		}
	}

	// Анкеты заполнены по деструкторам единицами, критической зоны быть
	// не должно; проверяем, что признак вообще считается.
	if _, ok := row["hasCritical"]; !ok {
		t.Error("в строке нет признака критической зоны")
	}
}

func TestСводкаНеОтдаётСырыхДанных(t *testing.T) {
	ts, mailer, st := newTestServer(t)
	hr := login(t, ts, mailer, "hr@example.com")
	createOrg(t, hr, ts, "Компас")
	member := addMember(t, hr, ts, "lead@example.com")

	leaderID, _ := member["leaderId"].(string)
	assessment, _ := st.CreateAssessment(t.Context(), leaderID, "Раунд")
	submitResponses(t, st, assessment.ID,
		[]domain.Role{domain.RolePeer, domain.RoleSubordinate, domain.RoleManager}, 4)
	grantConsent(t, ts, mailer, "lead@example.com")

	raw, _ := json.Marshal(overview(t, hr, ts))
	// Эйчар видит агрегаты, как и сам руководитель: по сырым ответам
	// восстанавливается, кто из команды что написал.
	for _, leak := range []string{"answers", "itemIndex", "tenure", "openAnswers", "responses"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("в сводке есть %q — это сырые данные анкет", leak)
		}
	}
}

func TestСводкаТолькоДляЭйчара(t *testing.T) {
	ts, mailer, st := newTestServer(t)
	hr := login(t, ts, mailer, "hr@example.com")
	createOrg(t, hr, ts, "Компас")
	addMember(t, hr, ts, "lead@example.com")

	// Обычный участник организации сводку видеть не должен: это набор
	// профилей его коллег.
	lead := login(t, ts, mailer, "lead@example.com")
	resp, _ := doJSON(t, lead, http.MethodGet, ts.URL+"/api/hr/overview", "")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("участник получил сводку: статус %d, ожидался 403", resp.StatusCode)
	}
	resp, _ = doJSON(t, lead, http.MethodPost, ts.URL+"/api/hr/members", `{"email":"x@example.com"}`)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("участник добавил кого-то: статус %d, ожидался 403", resp.StatusCode)
	}

	// Человек вне организации — 404: скрывать нечего, но и показывать нечего.
	outsider := login(t, ts, mailer, "outsider@example.com")
	resp, _ = doJSON(t, outsider, http.MethodGet, ts.URL+"/api/hr/overview", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("посторонний: статус %d, ожидался 404", resp.StatusCode)
	}

	_ = st
}

func TestЧужиеОрганизацииНеСмешиваются(t *testing.T) {
	ts, mailer, _ := newTestServer(t)

	first := login(t, ts, mailer, "hr1@example.com")
	createOrg(t, first, ts, "Первая")
	addMember(t, first, ts, "lead1@example.com")

	second := login(t, ts, mailer, "hr2@example.com")
	createOrg(t, second, ts, "Вторая")

	body := overview(t, second, ts)
	rows, _ := body["leaders"].([]any)
	if len(rows) != 1 {
		t.Fatalf("во второй организации %d строк, ожидалась одна", len(rows))
	}

	// Переманить участника чужой организации нельзя — иначе доступ к чужой
	// сводке получался бы простым добавлением.
	resp, _ := doJSON(t, second, http.MethodPost, ts.URL+"/api/hr/members", `{"email":"lead1@example.com"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("статус %d, ожидался 409", resp.StatusCode)
	}
}

func TestНекорректныеЗапросыHR(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	hr := login(t, ts, mailer, "hr@example.com")

	resp, _ := doJSON(t, hr, http.MethodPost, ts.URL+"/api/hr/org", `{"name":"   "}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("организация без названия: статус %d, ожидался 400", resp.StatusCode)
	}

	createOrg(t, hr, ts, "Компас")
	resp, _ = doJSON(t, hr, http.MethodPost, ts.URL+"/api/hr/org", `{"name":"Вторая"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("вторая организация: статус %d, ожидался 409", resp.StatusCode)
	}

	cases := map[string]string{
		"кривая почта":     `{"email":"не почта"}`,
		"неизвестная роль": `{"email":"x@example.com","role":"начальник"}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			resp, _ := doJSON(t, hr, http.MethodPost, ts.URL+"/api/hr/members", payload)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("статус %d, ожидался 400", resp.StatusCode)
			}
		})
	}
}

func TestHRТребуетВхода(t *testing.T) {
	ts, _, _ := newTestServer(t)
	client := newClient(t)

	paths := []struct{ method, path, body string }{
		{http.MethodGet, "/api/hr/overview", ""},
		{http.MethodPost, "/api/hr/org", `{"name":"Компас"}`},
		{http.MethodPost, "/api/hr/members", `{"email":"x@example.com"}`},
	}
	for _, p := range paths {
		resp, _ := doJSON(t, client, p.method, ts.URL+p.path, p.body)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: статус %d, ожидался 401", p.method, p.path, resp.StatusCode)
		}
	}
	_ = store.OrgRoleHR
}
