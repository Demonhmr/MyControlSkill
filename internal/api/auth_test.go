package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mycontrolskill/internal/store"
)

// recordingMailer запоминает отправленное вместо отправки.
type recordingMailer struct {
	mu     sync.Mutex
	logins []struct{ Email, Link string }
}

func (m *recordingMailer) SendLoginLink(ctx context.Context, email, link string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logins = append(m.logins, struct{ Email, Link string }{email, link})
	return nil
}

func (m *recordingMailer) SendInvite(ctx context.Context, email, link, leaderName string) error {
	return nil
}

func (m *recordingMailer) lastLink(t *testing.T) string {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.logins) == 0 {
		t.Fatal("ни одного письма не отправлено")
	}
	return m.logins[len(m.logins)-1].Link
}

func newTestServer(t *testing.T) (*httptest.Server, *recordingMailer, *store.Store) {
	t.Helper()
	ts, mailer, st, _ := newConfiguredServer(t)
	return ts, mailer, st
}

// newConfiguredServer дополнительно отдаёт сам обработчик: часть проверок
// меняет его настройки на ходу, например список допущенных.
func newConfiguredServer(t *testing.T) (*httptest.Server, *recordingMailer, *store.Store, *Server) {
	t.Helper()
	ctx := context.Background()

	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	mailer := &recordingMailer{}
	srv := &Server{
		Store:  st,
		Mailer: mailer,
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	mux := http.NewServeMux()
	srv.Register(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, mailer, st, srv
}

// client не ходит по редиректам: тестам нужно видеть сам ответ с cookie.
func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar := &cookieJar{}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// cookieJar — минимальная банка, хранящая cookie без учёта домена:
// в тестах адрес всегда один.
type cookieJar struct {
	mu      sync.Mutex
	cookies []*http.Cookie
}

func (j *cookieJar) SetCookies(_ *url.URL, cookies []*http.Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, c := range cookies {
		replaced := false
		for i, existing := range j.cookies {
			if existing.Name == c.Name {
				j.cookies[i] = c
				replaced = true
				break
			}
		}
		if !replaced {
			j.cookies = append(j.cookies, c)
		}
	}
}

func (j *cookieJar) Cookies(*url.URL) []*http.Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	var out []*http.Cookie
	for _, c := range j.cookies {
		if c.MaxAge < 0 || c.Value == "" {
			continue
		}
		out = append(out, c)
	}
	return out
}

func requestLogin(t *testing.T, ts *httptest.Server, client *http.Client, email string) *http.Response {
	t.Helper()
	body := strings.NewReader(`{"email":` + quote(email) + `}`)
	resp, err := client.Post(ts.URL+"/api/auth/login", "application/json", body)
	if err != nil {
		t.Fatalf("POST /api/auth/login: %v", err)
	}
	return resp
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestПолныйЦиклВхода(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := newClient(t)

	// До входа личный кабинет недоступен.
	resp, err := client.Get(ts.URL + "/api/me")
	if err != nil {
		t.Fatalf("GET /api/me: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("без сессии /api/me вернул %d, ожидался 401", resp.StatusCode)
	}

	resp = requestLogin(t, ts, client, "lead@example.com")
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("запрос ссылки вернул %d, ожидался 202", resp.StatusCode)
	}

	// Ссылка ушла «письмом» — переходим по ней.
	link := mailer.lastLink(t)
	if !strings.Contains(link, "/api/auth/callback?token=") {
		t.Fatalf("в письме неожиданная ссылка: %q", link)
	}

	resp, err = client.Get(link)
	if err != nil {
		t.Fatalf("переход по ссылке: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("переход по ссылке вернул %d, ожидался 303", resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "/" {
		t.Errorf("редирект на %q, ожидался /", got)
	}

	assertSessionCookie(t, resp)

	// Теперь личный кабинет доступен.
	resp, err = client.Get(ts.URL + "/api/me")
	if err != nil {
		t.Fatalf("GET /api/me: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/me вернул %d, ожидался 200", resp.StatusCode)
	}

	var me map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		t.Fatalf("разбор /api/me: %v", err)
	}
	if me["email"] != "lead@example.com" {
		t.Errorf("почта = %v", me["email"])
	}
	if me["id"] == "" || me["id"] == nil {
		t.Error("в ответе нет идентификатора")
	}
}

func assertSessionCookie(t *testing.T, resp *http.Response) {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name != sessionCookie {
			continue
		}
		if c.Value == "" {
			t.Error("cookie сессии пуста")
		}
		if !c.HttpOnly {
			t.Error("cookie сессии без HttpOnly — её украдёт любой XSS")
		}
		if c.SameSite != http.SameSiteLaxMode {
			t.Errorf("SameSite = %v, ожидался Lax: переход из почты межсайтовый", c.SameSite)
		}
		if c.Path != "/" {
			t.Errorf("Path = %q", c.Path)
		}
		return
	}
	t.Fatal("cookie сессии не выставлена")
}

func TestСсылкаВходаОдноразоваяЧерезHTTP(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := newClient(t)

	requestLogin(t, ts, client, "lead@example.com").Body.Close()
	link := mailer.lastLink(t)

	first, err := client.Get(link)
	if err != nil {
		t.Fatalf("первый переход: %v", err)
	}
	first.Body.Close()

	// Повторный переход по той же ссылке не должен пускать: письмо могло
	// уехать дальше по переписке.
	second, err := client.Get(link)
	if err != nil {
		t.Fatalf("повторный переход: %v", err)
	}
	second.Body.Close()

	loc := second.Header.Get("Location")
	if !strings.Contains(loc, "login_error=used") {
		t.Errorf("повторный переход отправил на %q, ожидалась пометка used", loc)
	}
}

func TestНеверныйТокенНеПускает(t *testing.T) {
	ts, _, _ := newTestServer(t)
	client := newClient(t)

	cases := map[string]string{
		"":                 "no-token",
		"выдуманный-токен": "invalid",
	}
	for token, wantReason := range cases {
		resp, err := client.Get(ts.URL + "/api/auth/callback?token=" + url.QueryEscape(token))
		if err != nil {
			t.Fatalf("переход с токеном %q: %v", token, err)
		}
		resp.Body.Close()

		loc := resp.Header.Get("Location")
		if !strings.Contains(loc, "login_error="+wantReason) {
			t.Errorf("токен %q: редирект на %q, ожидалась пометка %q", token, loc, wantReason)
		}
		for _, c := range resp.Cookies() {
			if c.Name == sessionCookie && c.Value != "" {
				t.Errorf("токен %q: выдана сессия при отказе", token)
			}
		}
	}
}

func TestВыходГаситСессию(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := newClient(t)

	requestLogin(t, ts, client, "lead@example.com").Body.Close()
	resp, err := client.Get(mailer.lastLink(t))
	if err != nil {
		t.Fatalf("вход: %v", err)
	}
	resp.Body.Close()

	out, err := client.Post(ts.URL+"/api/auth/logout", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/auth/logout: %v", err)
	}
	out.Body.Close()
	if out.StatusCode != http.StatusNoContent {
		t.Fatalf("выход вернул %d, ожидался 204", out.StatusCode)
	}

	me, err := client.Get(ts.URL + "/api/me")
	if err != nil {
		t.Fatalf("GET /api/me: %v", err)
	}
	me.Body.Close()
	if me.StatusCode != http.StatusUnauthorized {
		t.Errorf("после выхода /api/me вернул %d, ожидался 401", me.StatusCode)
	}
}

func TestПодобраннаяCookieНеПускает(t *testing.T) {
	ts, _, _ := newTestServer(t)

	bare := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	for _, value := range []string{"", "подобранное-значение", strings.Repeat("a", 43)} {
		req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/me", nil)
		if err != nil {
			t.Fatalf("сборка запроса: %v", err)
		}
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: value})

		resp, err := bare.Do(req)
		if err != nil {
			t.Fatalf("GET /api/me: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("cookie %q дала статус %d, ожидался 401", value, resp.StatusCode)
		}
	}
}

func TestНекорректныйЗапросВхода(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := newClient(t)

	for _, email := range []string{"", "не почта", "@example.com", "lead@"} {
		resp := requestLogin(t, ts, client, email)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("адрес %q: статус %d, ожидался 400", email, resp.StatusCode)
		}
	}

	// Битое тело тоже не должно валить сервер.
	resp, err := client.Post(ts.URL+"/api/auth/login", "application/json", strings.NewReader("{битый"))
	if err != nil {
		t.Fatalf("POST с битым телом: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("битое тело: статус %d, ожидался 400", resp.StatusCode)
	}

	mailer.mu.Lock()
	sent := len(mailer.logins)
	mailer.mu.Unlock()
	if sent != 0 {
		t.Errorf("на некорректные запросы отправлено писем: %d", sent)
	}
}

func TestОтветыAPIНеКэшируются(t *testing.T) {
	ts, _, _ := newTestServer(t)
	client := newClient(t)

	resp, err := client.Get(ts.URL + "/api/me")
	if err != nil {
		t.Fatalf("GET /api/me: %v", err)
	}
	resp.Body.Close()

	// Ответы персональные: их кэширование прокси означало бы выдачу чужих
	// данных следующему запросившему.
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, ожидался no-store", got)
	}
}
