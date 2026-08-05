package api

import (
	"net/http"
	"strings"
	"testing"
)

// onlyCompany — список допущенных: один домен.
func onlyCompany(email string) bool { return strings.HasSuffix(email, "@company.ru") }

func TestПосторонниеНеПолучаютСсылку(t *testing.T) {
	ts, mailer, _, server := newConfiguredServer(t)
	server.AllowRegistration = onlyCompany

	client := newClient(t)
	resp := requestLogin(t, ts, client, "stranger@example.com")
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("статус %d, ожидался 403", resp.StatusCode)
	}

	// Письма быть не должно: отправлять ссылку тому, кто всё равно не
	// сможет войти, бессмысленно.
	mailer.mu.Lock()
	sent := len(mailer.logins)
	mailer.mu.Unlock()
	if sent != 0 {
		t.Errorf("отправлено писем: %d", sent)
	}
}

func TestДопущенныеВходятКакРаньше(t *testing.T) {
	ts, mailer, _, server := newConfiguredServer(t)
	server.AllowRegistration = onlyCompany

	client := login(t, ts, mailer, "lead@company.ru")
	resp, body := doJSON(t, client, http.MethodGet, ts.URL+"/api/me", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("статус %d, ожидался 200", resp.StatusCode)
	}
	if body["email"] != "lead@company.ru" {
		t.Errorf("почта = %v", body["email"])
	}
}

// Ссылка могла быть выдана до того, как список сузили. Гасить её на входе
// мало — аккаунт по ней заводиться не должен.
func TestСтараяСсылкаНеОбходитСписок(t *testing.T) {
	ts, mailer, _, server := newConfiguredServer(t)

	// Пока список пуст, ссылка выдаётся кому угодно.
	client := newClient(t)
	requestLogin(t, ts, client, "stranger@example.com").Body.Close()
	link := mailer.lastLink(t)

	// Список сузили уже после выдачи.
	server.AllowRegistration = onlyCompany

	resp, err := client.Get(link)
	if err != nil {
		t.Fatalf("переход по ссылке: %v", err)
	}
	resp.Body.Close()

	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "login_error=not-allowed") {
		t.Errorf("редирект на %q, ожидалась пометка not-allowed", loc)
	}
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			t.Error("выдана сессия постороннему")
		}
	}
}

func TestДобавленныйЭйчаромВходитМимоСписка(t *testing.T) {
	ts, mailer, st, server := newConfiguredServer(t)

	hr := login(t, ts, mailer, "hr@company.ru")
	createOrg(t, hr, ts, "Компас")
	// Эйчар добавляет подрядчика с почтой вне списка — это осознанное
	// действие доверенного человека, и аккаунт заводится сразу.
	addMember(t, hr, ts, "contractor@outside.com")

	server.AllowRegistration = onlyCompany

	if _, err := st.LeaderByEmail(t.Context(), "contractor@outside.com"); err != nil {
		t.Fatalf("аккаунт подрядчика не заведён: %v", err)
	}

	// Весь путь целиком, а не только хранилище: запрос ссылки, переход,
	// личный кабинет. Проверка одного лишь хранилища скрыла бы отказ на
	// самом первом шаге.
	contractor := login(t, ts, mailer, "contractor@outside.com")
	resp, body := doJSON(t, contractor, http.MethodGet, ts.URL+"/api/me", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("подрядчик не вошёл: статус %d", resp.StatusCode)
	}
	if body["email"] != "contractor@outside.com" {
		t.Errorf("почта = %v", body["email"])
	}
}

func TestБезСпискаПоведениеПрежнее(t *testing.T) {
	ts, mailer, _ := newTestServer(t)

	// AllowRegistration не задан: сервис, поднятый без настройки, работает
	// как раньше.
	client := login(t, ts, mailer, "anyone@example.com")
	resp, _ := doJSON(t, client, http.MethodGet, ts.URL+"/api/me", "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("статус %d, ожидался 200", resp.StatusCode)
	}
}
