package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mycontrolskill/internal/domain"
	"mycontrolskill/internal/store"
)

// login выполняет полный вход и возвращает клиента с сессией.
func login(t *testing.T, ts *httptest.Server, mailer *recordingMailer, email string) *http.Client {
	t.Helper()
	client := newClient(t)
	requestLogin(t, ts, client, email).Body.Close()

	resp, err := client.Get(mailer.lastLink(t))
	if err != nil {
		t.Fatalf("вход: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("вход вернул %d", resp.StatusCode)
	}
	return client
}

func doJSON(t *testing.T, client *http.Client, method, url, body string) (*http.Response, map[string]any) {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("сборка запроса: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("чтение ответа: %v", err)
	}
	var decoded map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("разбор ответа %s: %v", raw, err)
		}
	}
	return resp, decoded
}

// createAssessment заводит раунд через API и возвращает его идентификатор.
func createAssessment(t *testing.T, client *http.Client, ts *httptest.Server, title string) string {
	t.Helper()
	resp, body := doJSON(t, client, http.MethodPost, ts.URL+"/api/assessments", `{"title":`+quote(title)+`}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("создание раунда вернуло %d: %v", resp.StatusCode, body)
	}
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatalf("в ответе нет идентификатора: %v", body)
	}
	return id
}

func TestРаундСоздаётсяИПопадаетВСписок(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")

	id := createAssessment(t, client, ts, "Раунд 1")

	resp, body := doJSON(t, client, http.MethodGet, ts.URL+"/api/assessments", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("список вернул %d", resp.StatusCode)
	}
	list, _ := body["assessments"].([]any)
	if len(list) != 1 {
		t.Fatalf("раундов в списке %d, ожидался один", len(list))
	}

	first, _ := list[0].(map[string]any)
	if first["id"] != id {
		t.Errorf("в списке чужой раунд: %v", first["id"])
	}
	counts, _ := first["counts"].(map[string]any)
	if counts["required"] != float64(domain.MinRespondents) {
		t.Errorf("порог в ответе = %v, ожидался %d", counts["required"], domain.MinRespondents)
	}
	if counts["ready"] != false {
		t.Errorf("пустой раунд помечен готовым")
	}
}

func TestПриглашениеОтдаётСсылкуОдинРаз(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")
	id := createAssessment(t, client, ts, "Раунд")

	resp, body := doJSON(t, client, http.MethodPost,
		ts.URL+"/api/assessments/"+id+"/invites", `{"role":"peer","email":"peer@example.com"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("создание приглашения вернуло %d: %v", resp.StatusCode, body)
	}

	link, _ := body["link"].(string)
	if !strings.Contains(link, surveyPath) {
		t.Errorf("ссылка на анкету неожиданного вида: %q", link)
	}
	token := link[strings.Index(link, surveyPath)+len(surveyPath):]
	if len(token) < 40 {
		t.Errorf("подозрительно короткий токен в ссылке: %q", token)
	}

	invite, _ := body["invite"].(map[string]any)
	if _, leaked := invite["token"]; leaked {
		t.Error("токен попал в представление приглашения")
	}
	if invite["usedAt"] != nil {
		t.Errorf("новое приглашение помечено использованным: %v", invite["usedAt"])
	}

	// В списке приглашений токена быть не должно ни в каком виде.
	resp, body = doJSON(t, client, http.MethodGet, ts.URL+"/api/assessments/"+id, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("чтение раунда вернуло %d", resp.StatusCode)
	}
	raw, _ := json.Marshal(body)
	if strings.Contains(string(raw), token) {
		t.Error("токен приглашения виден в списке приглашений")
	}
	invites, _ := body["invites"].([]any)
	if len(invites) != 1 {
		t.Errorf("приглашений в списке %d, ожидалось одно", len(invites))
	}
}

func TestНекорректноеПриглашениеОтвергается(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")
	id := createAssessment(t, client, ts, "Раунд")

	cases := map[string]string{
		"неизвестная роль": `{"role":"начальник"}`,
		"пустая роль":      `{"role":""}`,
		"битая почта":      `{"role":"peer","email":"не почта"}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			resp, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/assessments/"+id+"/invites", payload)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("статус %d, ожидался 400", resp.StatusCode)
			}
		})
	}

	// Приглашение без почты допустимо: ссылку можно передать и вне почты.
	resp, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/assessments/"+id+"/invites", `{"role":"peer"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("приглашение без почты вернуло %d, ожидался 201", resp.StatusCode)
	}
}

func TestЗакрытыйРаундНеПринимаетПриглашения(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")
	id := createAssessment(t, client, ts, "Раунд")

	resp, body := doJSON(t, client, http.MethodPost, ts.URL+"/api/assessments/"+id+"/close", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("закрытие вернуло %d", resp.StatusCode)
	}
	if body["closedAt"] == nil {
		t.Error("после закрытия closedAt пуст")
	}

	resp, _ = doJSON(t, client, http.MethodPost, ts.URL+"/api/assessments/"+id+"/invites", `{"role":"peer"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("приглашение в закрытый раунд вернуло %d, ожидался 409", resp.StatusCode)
	}
}

// Чужой раунд должен выглядеть как несуществующий: по разнице между 403 и
// 404 перебором идентификаторов выяснялось бы, какие раунды есть у других.
func TestЧужойРаундНеВиден(t *testing.T) {
	ts, mailer, _ := newTestServer(t)

	owner := login(t, ts, mailer, "owner@example.com")
	id := createAssessment(t, owner, ts, "Чужой раунд")

	stranger := login(t, ts, mailer, "stranger@example.com")

	paths := []struct{ method, path string }{
		{http.MethodGet, "/api/assessments/" + id},
		{http.MethodGet, "/api/assessments/" + id + "/profile"},
		{http.MethodPost, "/api/assessments/" + id + "/close"},
		{http.MethodPost, "/api/assessments/" + id + "/invites"},
	}
	for _, p := range paths {
		body := ""
		if p.method == http.MethodPost && strings.HasSuffix(p.path, "/invites") {
			body = `{"role":"peer"}`
		}
		resp, _ := doJSON(t, stranger, p.method, ts.URL+p.path, body)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s %s: статус %d, ожидался 404", p.method, p.path, resp.StatusCode)
		}
	}

	// Список чужих раундов тоже пуст.
	_, body := doJSON(t, stranger, http.MethodGet, ts.URL+"/api/assessments", "")
	if list, _ := body["assessments"].([]any); len(list) != 0 {
		t.Errorf("в списке постороннего %d раундов", len(list))
	}
}

func TestБезСессииРаундыНедоступны(t *testing.T) {
	ts, _, _ := newTestServer(t)
	client := newClient(t)

	paths := []struct{ method, path string }{
		{http.MethodGet, "/api/assessments"},
		{http.MethodPost, "/api/assessments"},
		{http.MethodGet, "/api/assessments/любой/profile"},
		{http.MethodPost, "/api/assessments/любой/invites"},
	}
	for _, p := range paths {
		resp, _ := doJSON(t, client, p.method, ts.URL+p.path, "{}")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: статус %d, ожидался 401", p.method, p.path, resp.StatusCode)
		}
	}
}

// submitResponses наполняет раунд анкетами напрямую через хранилище:
// HTTP-приёмки анкет ещё нет, она появится вместе с экраном респондента.
func submitResponses(t *testing.T, st *store.Store, assessmentID string, roles []domain.Role, value int) {
	t.Helper()
	ctx := context.Background()

	for _, role := range roles {
		_, token, err := st.CreateInvite(ctx, assessmentID, role, "")
		if err != nil {
			t.Fatalf("CreateInvite: %v", err)
		}
		sub := domain.Submission{Tenure: domain.TenureOver1Year}
		for _, code := range domain.CompetencyCodes {
			for i := 0; i < domain.ItemsPerCode; i++ {
				v := value
				sub.Answers = append(sub.Answers, domain.Answer{
					Kind: domain.KindCompetency, Code: code, ItemIndex: i, Value: &v,
				})
			}
		}
		if _, err := st.SubmitByToken(ctx, token, sub); err != nil {
			t.Fatalf("SubmitByToken: %v", err)
		}
	}
}

func TestПрофильНедоступенНижеПорога(t *testing.T) {
	ts, mailer, st := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")
	id := createAssessment(t, client, ts, "Раунд")

	submitResponses(t, st, id, []domain.Role{domain.RolePeer, domain.RoleSubordinate}, 5)

	resp, body := doJSON(t, client, http.MethodGet, ts.URL+"/api/assessments/"+id+"/profile", "")
	if resp.StatusCode != http.StatusLocked {
		t.Fatalf("статус %d, ожидался 423", resp.StatusCode)
	}

	// Ни одного перцентиля в ответе быть не должно — только счётчики.
	raw, _ := json.Marshal(body)
	if strings.Contains(string(raw), "percentile") {
		t.Errorf("ниже порога отдан перцентиль: %s", raw)
	}
	counts, _ := body["counts"].(map[string]any)
	if counts["external"] != float64(2) {
		t.Errorf("внешних анкет в ответе = %v, ожидалось 2", counts["external"])
	}
	if counts["ready"] != false {
		t.Error("счётчики помечены готовыми ниже порога")
	}
}

func TestПрофильСчитаетсяПослеПорога(t *testing.T) {
	ts, mailer, st := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")
	id := createAssessment(t, client, ts, "Раунд")

	submitResponses(t, st, id,
		[]domain.Role{domain.RolePeer, domain.RoleSubordinate, domain.RoleManager, domain.RoleSelf}, 5)

	resp, body := doJSON(t, client, http.MethodGet, ts.URL+"/api/assessments/"+id+"/profile", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("статус %d, ожидался 200: %v", resp.StatusCode, body)
	}

	profile, _ := body["profile"].(map[string]any)
	if profile["ready"] != true {
		t.Error("профиль не помечен готовым")
	}
	if profile["respondentCount"] != float64(3) {
		t.Errorf("внешних анкет = %v, ожидалось 3 (самооценка не в счёт)", profile["respondentCount"])
	}

	competencies, _ := profile["competencies"].([]any)
	if len(competencies) != len(domain.CompetencyCodes) {
		t.Fatalf("компетенций в ответе %d, ожидалось %d", len(competencies), len(domain.CompetencyCodes))
	}
	first, _ := competencies[0].(map[string]any)
	if first["percentile"] == nil {
		t.Error("перцентиль не посчитан выше порога")
	}

	// Сырые ответы наружу уходить не должны ни в каком виде: по ним
	// восстанавливается, кто именно как отвечал.
	raw, _ := json.Marshal(body)
	for _, leak := range []string{"answers", "itemIndex", "tenure", "responses"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("в ответе есть %q — это сырые данные анкет", leak)
		}
	}
}
