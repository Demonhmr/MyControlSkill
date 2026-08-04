package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mycontrolskill/internal/domain"
)

// inviteLink создаёт приглашение через API и возвращает токен из ссылки.
func inviteToken(t *testing.T, client *http.Client, ts *httptest.Server, assessmentID, role string) string {
	t.Helper()
	resp, body := doJSON(t, client, http.MethodPost,
		ts.URL+"/api/assessments/"+assessmentID+"/invites", `{"role":`+quote(role)+`}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("создание приглашения вернуло %d: %v", resp.StatusCode, body)
	}
	link, _ := body["link"].(string)
	return link[strings.Index(link, surveyPath)+len(surveyPath):]
}

// fullSurveyJSON собирает тело заполненной анкеты.
func fullSurveyJSON(t *testing.T, tenure string, value any) string {
	t.Helper()
	type answer struct {
		Kind      string `json:"kind"`
		Code      string `json:"code"`
		ItemIndex int    `json:"itemIndex"`
		Value     any    `json:"value"`
	}
	var answers []answer
	for _, code := range domain.CompetencyCodes {
		for i := 0; i < domain.ItemsPerCode; i++ {
			answers = append(answers, answer{"competency", code, i, value})
		}
	}
	for _, code := range domain.DestructorCodes {
		for i := 0; i < domain.ItemsPerCode; i++ {
			answers = append(answers, answer{"destructor", code, i, value})
		}
	}

	body := map[string]any{
		"tenure":  tenure,
		"answers": answers,
		"openAnswers": []map[string]any{
			{"questionIndex": 0, "text": "Разобрал провал спокойно и по делу."},
			{"questionIndex": 1, "text": "  "},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("сборка анкеты: %v", err)
	}
	return string(raw)
}

func TestАнкетаОткрываетсяПоСсылкеБезВхода(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	leadClient := login(t, ts, mailer, "lead@example.com")
	id := createAssessment(t, leadClient, ts, "Раунд")
	token := inviteToken(t, leadClient, ts, id, "subordinate")

	// Респондент приходит без сессии.
	anon := newClient(t)
	resp, body := doJSON(t, anon, http.MethodGet, ts.URL+"/api/survey/"+token, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("статус %d, ожидался 200", resp.StatusCode)
	}
	if body["role"] != "subordinate" {
		t.Errorf("роль = %v, ожидалась subordinate из приглашения", body["role"])
	}
	if body["leaderName"] != "lead@example.com" {
		t.Errorf("имя оцениваемого = %v, ожидалась подстановка почты", body["leaderName"])
	}
	if body["used"] != false || body["closed"] != false {
		t.Errorf("свежая ссылка помечена использованной или закрытой: %v", body)
	}
}

func TestНеизвестнаяСсылкаНаАнкету(t *testing.T) {
	ts, _, _ := newTestServer(t)
	anon := newClient(t)

	resp, _ := doJSON(t, anon, http.MethodGet, ts.URL+"/api/survey/выдуманный", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET: статус %d, ожидался 404", resp.StatusCode)
	}

	resp, _ = doJSON(t, anon, http.MethodPost, ts.URL+"/api/survey/выдуманный", fullSurveyJSON(t, "gt12", 4))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("POST: статус %d, ожидался 404", resp.StatusCode)
	}
}

func TestАнкетаОтправляетсяИСсылкаГаснет(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	leadClient := login(t, ts, mailer, "lead@example.com")
	id := createAssessment(t, leadClient, ts, "Раунд")
	token := inviteToken(t, leadClient, ts, id, "peer")

	anon := newClient(t)
	resp, _ := doJSON(t, anon, http.MethodPost, ts.URL+"/api/survey/"+token, fullSurveyJSON(t, "gt12", 4))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("отправка вернула %d, ожидался 201", resp.StatusCode)
	}

	// Повторная отправка по той же ссылке отклоняется.
	resp, _ = doJSON(t, anon, http.MethodPost, ts.URL+"/api/survey/"+token, fullSurveyJSON(t, "gt12", 5))
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("повторная отправка вернула %d, ожидался 409", resp.StatusCode)
	}

	// А контекст анкеты теперь помечен использованным — экрану надо это
	// показать, а не выдать пустую страницу.
	resp, body := doJSON(t, anon, http.MethodGet, ts.URL+"/api/survey/"+token, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET после отправки вернул %d", resp.StatusCode)
	}
	if body["used"] != true {
		t.Errorf("ссылка не помечена использованной: %v", body)
	}

	// Анкета доехала до счётчиков руководителя.
	_, leadBody := doJSON(t, leadClient, http.MethodGet, ts.URL+"/api/assessments/"+id, "")
	assessment, _ := leadBody["assessment"].(map[string]any)
	counts, _ := assessment["counts"].(map[string]any)
	if counts["external"] != float64(1) {
		t.Errorf("внешних анкет у руководителя = %v, ожидалась одна", counts["external"])
	}
}

func TestАнкетаВЗакрытыйРаундНеПринимается(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	leadClient := login(t, ts, mailer, "lead@example.com")
	id := createAssessment(t, leadClient, ts, "Раунд")
	token := inviteToken(t, leadClient, ts, id, "peer")

	doJSON(t, leadClient, http.MethodPost, ts.URL+"/api/assessments/"+id+"/close", "")

	anon := newClient(t)
	resp, _ := doJSON(t, anon, http.MethodPost, ts.URL+"/api/survey/"+token, fullSurveyJSON(t, "gt12", 4))
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("отправка в закрытый раунд вернула %d, ожидался 409", resp.StatusCode)
	}

	_, body := doJSON(t, anon, http.MethodGet, ts.URL+"/api/survey/"+token, "")
	if body["closed"] != true {
		t.Errorf("раунд не помечен закрытым: %v", body)
	}
}

func TestНекорректнаяАнкетаОтвергается(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	leadClient := login(t, ts, mailer, "lead@example.com")
	id := createAssessment(t, leadClient, ts, "Раунд")

	cases := map[string]string{
		"неизвестный срок наблюдения": `{"tenure":"давно","answers":[]}`,
		"пустой срок":                 `{"tenure":"","answers":[]}`,
		"оценка вне шкалы":            `{"tenure":"gt12","answers":[{"kind":"competency","code":"COM","itemIndex":0,"value":9}]}`,
		"неизвестный код":             `{"tenure":"gt12","answers":[{"kind":"competency","code":"НЕТ","itemIndex":0,"value":4}]}`,
		"пункт вне диапазона":         `{"tenure":"gt12","answers":[{"kind":"competency","code":"COM","itemIndex":7,"value":4}]}`,
		"дубль пункта":                `{"tenure":"gt12","answers":[{"kind":"competency","code":"COM","itemIndex":0,"value":4},{"kind":"competency","code":"COM","itemIndex":0,"value":5}]}`,
		"битое тело":                  `{не json`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			// Каждому случаю своя ссылка: неудачная попытка не должна её гасить.
			token := inviteToken(t, leadClient, ts, id, "peer")

			resp, _ := doJSON(t, newClient(t), http.MethodPost, ts.URL+"/api/survey/"+token, payload)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("статус %d, ожидался 400", resp.StatusCode)
			}

			// Ссылка осталась рабочей.
			_, body := doJSON(t, newClient(t), http.MethodGet, ts.URL+"/api/survey/"+token, "")
			if body["used"] != false {
				t.Error("отклонённая анкета погасила ссылку")
			}
		})
	}
}

func TestСлишкомДлинныйОткрытыйОтвет(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	leadClient := login(t, ts, mailer, "lead@example.com")
	id := createAssessment(t, leadClient, ts, "Раунд")
	token := inviteToken(t, leadClient, ts, id, "peer")

	long := strings.Repeat("я", domain.MaxOpenAnswerLength+1)
	payload := `{"tenure":"gt12","answers":[],"openAnswers":[{"questionIndex":0,"text":` + quote(long) + `}]}`

	resp, _ := doJSON(t, newClient(t), http.MethodPost, ts.URL+"/api/survey/"+token, payload)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("статус %d, ожидался 400", resp.StatusCode)
	}
}

// Роль назначает руководитель; попытка прислать свою не должна ничего менять.
func TestРольИзТелаЗапросаИгнорируется(t *testing.T) {
	ts, mailer, st := newTestServer(t)
	leadClient := login(t, ts, mailer, "lead@example.com")
	id := createAssessment(t, leadClient, ts, "Раунд")
	token := inviteToken(t, leadClient, ts, id, "subordinate")

	// Поле role в схеме запроса не описано, поэтому строгий разбор его
	// отклонит — это и есть нужное поведение.
	payload := `{"tenure":"gt12","answers":[],"role":"self"}`
	resp, _ := doJSON(t, newClient(t), http.MethodPost, ts.URL+"/api/survey/"+token, payload)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("лишнее поле role: статус %d, ожидался 400", resp.StatusCode)
	}

	// А корректная отправка сохраняется с ролью из приглашения.
	resp, _ = doJSON(t, newClient(t), http.MethodPost, ts.URL+"/api/survey/"+token, `{"tenure":"gt12","answers":[]}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("отправка вернула %d", resp.StatusCode)
	}

	stored, err := st.ResponsesForScoring(t.Context(), id)
	if err != nil {
		t.Fatalf("ResponsesForScoring: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("анкет сохранено %d", len(stored))
	}
	if stored[0].Role != domain.RoleSubordinate {
		t.Errorf("роль = %q, ожидалась subordinate", stored[0].Role)
	}
}

func TestПустойОткрытыйОтветНеСохраняется(t *testing.T) {
	ts, mailer, st := newTestServer(t)
	leadClient := login(t, ts, mailer, "lead@example.com")
	id := createAssessment(t, leadClient, ts, "Раунд")
	token := inviteToken(t, leadClient, ts, id, "peer")

	resp, _ := doJSON(t, newClient(t), http.MethodPost, ts.URL+"/api/survey/"+token, fullSurveyJSON(t, "gt12", 4))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("отправка вернула %d", resp.StatusCode)
	}

	// В анкете два открытых вопроса, второй заполнен пробелами: пустая
	// строка и пропущенный вопрос — это одно и то же, хранить нечего.
	n, err := st.CountOpenAnswers(t.Context(), id)
	if err != nil {
		t.Fatalf("CountOpenAnswers: %v", err)
	}
	if n != 1 {
		t.Errorf("сохранено открытых ответов %d, ожидался один", n)
	}
}
