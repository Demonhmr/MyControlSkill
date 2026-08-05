package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"mycontrolskill/internal/domain"
)

func TestУчастиеВидноСамомуРуководителю(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	hr := login(t, ts, mailer, "hr@example.com")
	createOrg(t, hr, ts, "ООО «Компас»")
	addMember(t, hr, ts, "lead@example.com")

	lead := login(t, ts, mailer, "lead@example.com")
	resp, body := doJSON(t, lead, http.MethodGet, ts.URL+"/api/me/org", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("статус %d, ожидался 200", resp.StatusCode)
	}

	org, _ := body["org"].(map[string]any)
	if org["name"] != "ООО «Компас»" {
		t.Errorf("организация = %v", org["name"])
	}
	if body["role"] != "leader" {
		t.Errorf("роль = %v", body["role"])
	}
	// Молчание согласием не считается: человека добавил эйчар, а не он сам.
	if body["consentGranted"] != false {
		t.Errorf("согласие по умолчанию = %v", body["consentGranted"])
	}
	if body["consentAt"] != nil {
		t.Errorf("момент согласия заполнен без согласия: %v", body["consentAt"])
	}
}

func TestБезОрганизацииУчастиеОтдаёт404(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	lone := login(t, ts, mailer, "lone@example.com")

	resp, _ := doJSON(t, lone, http.MethodGet, ts.URL+"/api/me/org", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("статус %d, ожидался 404", resp.StatusCode)
	}
	resp, _ = doJSON(t, lone, http.MethodPut, ts.URL+"/api/me/org/consent", `{"granted":true}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("согласие без организации: статус %d, ожидался 404", resp.StatusCode)
	}
}

// Главное свойство: без согласия эйчар не видит ни чисел, ни счётчиков —
// сколько анкет человек собрал, уже сведения о нём.
func TestБезСогласияЭйчарНеВидитЧисел(t *testing.T) {
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

	row := leaderRow(t, overview(t, hr, ts), "lead@example.com")
	if row["consentGranted"] != false {
		t.Fatalf("строка помечена согласованной: %v", row)
	}
	if row["ready"] != false {
		t.Error("без согласия строка помечена готовой")
	}
	if row["counts"] != nil {
		t.Errorf("без согласия видны счётчики: %v", row["counts"])
	}

	raw, _ := json.Marshal(row)
	if strings.Contains(string(raw), `"percentile"`) {
		t.Errorf("без согласия отдан перцентиль: %s", raw)
	}
}

func TestПослеСогласияЧислаПоявляютсяИИсчезаютПриОтзыве(t *testing.T) {
	ts, mailer, st := newTestServer(t)
	hr := login(t, ts, mailer, "hr@example.com")
	createOrg(t, hr, ts, "Компас")
	member := addMember(t, hr, ts, "lead@example.com")

	leaderID, _ := member["leaderId"].(string)
	assessment, _ := st.CreateAssessment(t.Context(), leaderID, "Раунд")
	submitResponses(t, st, assessment.ID,
		[]domain.Role{domain.RolePeer, domain.RoleSubordinate, domain.RoleManager}, 5)

	lead := login(t, ts, mailer, "lead@example.com")
	resp, body := doJSON(t, lead, http.MethodPut, ts.URL+"/api/me/org/consent", `{"granted":true}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("выдача согласия: статус %d", resp.StatusCode)
	}
	if body["consentGranted"] != true || body["consentAt"] == nil {
		t.Fatalf("согласие не зафиксировано: %v", body)
	}

	row := leaderRow(t, overview(t, hr, ts), "lead@example.com")
	if row["ready"] != true {
		t.Fatalf("после согласия числа не появились: %v", row)
	}

	// Отзыв должен действовать сразу: согласие отзывается тогда, когда
	// человек передумал, а не со следующего раунда.
	resp, _ = doJSON(t, lead, http.MethodPut, ts.URL+"/api/me/org/consent", `{"granted":false}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("отзыв согласия: статус %d", resp.StatusCode)
	}

	row = leaderRow(t, overview(t, hr, ts), "lead@example.com")
	if row["ready"] != false || row["consentGranted"] != false {
		t.Errorf("после отзыва числа остались: %v", row)
	}
	raw, _ := json.Marshal(row)
	if strings.Contains(string(raw), `"percentile"`) {
		t.Errorf("после отзыва отдан перцентиль: %s", raw)
	}
}

// Согласие нужно на показ другим, а не самому себе: свою строку эйчар
// видит без него, иначе пришлось бы согласовывать доступ к своим же данным.
func TestСвоюСтрокуЭйчарВидитБезСогласия(t *testing.T) {
	ts, mailer, st := newTestServer(t)
	hr := login(t, ts, mailer, "hr@example.com")
	createOrg(t, hr, ts, "Компас")

	_, me := doJSON(t, hr, http.MethodGet, ts.URL+"/api/me", "")
	hrID, _ := me["id"].(string)

	assessment, err := st.CreateAssessment(t.Context(), hrID, "Свой раунд")
	if err != nil {
		t.Fatalf("CreateAssessment: %v", err)
	}
	submitResponses(t, st, assessment.ID,
		[]domain.Role{domain.RolePeer, domain.RoleSubordinate, domain.RoleManager}, 4)

	row := leaderRow(t, overview(t, hr, ts), "hr@example.com")
	if row["ready"] != true {
		t.Errorf("эйчар не видит собственных чисел: %v", row)
	}
}

func TestСогласиеЗаДругогоНеВыдать(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	hr := login(t, ts, mailer, "hr@example.com")
	createOrg(t, hr, ts, "Компас")
	addMember(t, hr, ts, "lead@example.com")

	// Эндпоинт работает только от своего лица: параметра с чужим
	// идентификатором в нём нет вовсе, и лишнее поле отвергается разбором.
	resp, _ := doJSON(t, hr, http.MethodPut, ts.URL+"/api/me/org/consent",
		`{"granted":true,"leaderId":"чужой"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("статус %d, ожидался 400", resp.StatusCode)
	}

	// Согласие эйчара за себя на чужую строку не влияет.
	doJSON(t, hr, http.MethodPut, ts.URL+"/api/me/org/consent", `{"granted":true}`)
	row := leaderRow(t, overview(t, hr, ts), "lead@example.com")
	if row["consentGranted"] != false {
		t.Errorf("чужая строка согласована: %v", row)
	}
}

func TestСогласиеТребуетВхода(t *testing.T) {
	ts, _, _ := newTestServer(t)
	client := newClient(t)

	paths := []struct{ method, path, body string }{
		{http.MethodGet, "/api/me/org", ""},
		{http.MethodPut, "/api/me/org/consent", `{"granted":true}`},
	}
	for _, p := range paths {
		resp, _ := doJSON(t, client, p.method, ts.URL+p.path, p.body)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: статус %d, ожидался 401", p.method, p.path, resp.StatusCode)
		}
	}
}
