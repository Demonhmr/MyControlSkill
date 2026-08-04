package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestЗапросСсылокОграниченПоАдресу(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := newClient(t)

	// Дважды подряд — обычное дело: письмо не пришло, человек нажал ещё раз.
	for i := range loginEmailBurst {
		resp := requestLogin(t, ts, client, "lead@example.com")
		resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("попытка %d вернула %d, ожидался 202", i+1, resp.StatusCode)
		}
	}

	resp := requestLogin(t, ts, client, "lead@example.com")
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("статус %d, ожидался 429", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("нет заголовка Retry-After — клиенту неоткуда узнать, когда пробовать")
	}

	// Писем ушло ровно столько, сколько разрешено.
	mailer.mu.Lock()
	sent := len(mailer.logins)
	mailer.mu.Unlock()
	if sent != loginEmailBurst {
		t.Errorf("отправлено писем %d, ожидалось %d", sent, loginEmailBurst)
	}
}

func TestДругойАдресНеСтрадаетОтЧужогоПредела(t *testing.T) {
	ts, _, _ := newTestServer(t)
	client := newClient(t)

	for range loginEmailBurst + 1 {
		requestLogin(t, ts, client, "spammed@example.com").Body.Close()
	}

	resp := requestLogin(t, ts, client, "another@example.com")
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("чужой предел задел другой адрес: статус %d", resp.StatusCode)
	}
}

func TestРассылкаПоРазнымАдресамОграниченаПоИсточнику(t *testing.T) {
	ts, _, _ := newTestServer(t)
	client := newClient(t)

	// Предел по адресу обходится сменой адреса в каждом запросе — на это и
	// нужен отдельный предел по источнику.
	for i := range loginIPBurst {
		resp := requestLogin(t, ts, client, fmt.Sprintf("target-%d@example.com", i))
		resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("попытка %d вернула %d", i+1, resp.StatusCode)
		}
	}

	resp := requestLogin(t, ts, client, "target-last@example.com")
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("статус %d, ожидался 429", resp.StatusCode)
	}
}

func TestНекорректныйАдресНеТратитЛимит(t *testing.T) {
	ts, _, _ := newTestServer(t)
	client := newClient(t)

	// Опечатка в адресе не должна лишать человека права на попытку.
	for range loginIPBurst + 5 {
		requestLogin(t, ts, client, "не почта").Body.Close()
	}

	resp := requestLogin(t, ts, client, "lead@example.com")
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("после отказов по форме адреса статус %d, ожидался 202", resp.StatusCode)
	}
}

func TestВыдачаПриглашенийОграничена(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")
	id := createAssessment(t, client, ts, "Раунд")

	for i := range invitesBurst {
		resp, _ := doJSON(t, client, http.MethodPost,
			ts.URL+"/api/assessments/"+id+"/invites", `{"role":"peer"}`)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("приглашение %d вернуло %d", i+1, resp.StatusCode)
		}
	}

	resp, _ := doJSON(t, client, http.MethodPost,
		ts.URL+"/api/assessments/"+id+"/invites", `{"role":"peer"}`)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("статус %d, ожидался 429", resp.StatusCode)
	}
}

func TestАдресКлиента(t *testing.T) {
	cases := []struct {
		name       string
		trustProxy bool
		remoteAddr string
		forwarded  string
		want       string
	}{
		{
			name:       "без прокси берётся адрес соединения",
			remoteAddr: "203.0.113.7:54321",
			forwarded:  "198.51.100.1",
			want:       "203.0.113.7",
		},
		{
			// Иначе за прокси все запросы выглядят с одного адреса, и предел
			// по источнику либо бессмыслен, либо блокирует всех разом.
			name:       "с прокси берётся заголовок",
			trustProxy: true,
			remoteAddr: "10.0.0.1:54321",
			forwarded:  "198.51.100.1",
			want:       "198.51.100.1",
		},
		{
			// Клиент может прислать свой X-Forwarded-For; наш прокси
			// допишет настоящий адрес справа, поэтому берётся последний.
			name:       "подделанный заголовок не перебивает запись прокси",
			trustProxy: true,
			remoteAddr: "10.0.0.1:54321",
			forwarded:  "1.2.3.4, 198.51.100.1",
			want:       "198.51.100.1",
		},
		{
			name:       "пустой заголовок откатывается на адрес соединения",
			trustProxy: true,
			remoteAddr: "10.0.0.1:54321",
			forwarded:  "",
			want:       "10.0.0.1",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &Server{TrustProxy: c.trustProxy}
			r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
			r.RemoteAddr = c.remoteAddr
			if c.forwarded != "" {
				r.Header.Set("X-Forwarded-For", c.forwarded)
			}

			if got := s.clientIP(r); got != c.want {
				t.Errorf("clientIP = %q, ожидался %q", got, c.want)
			}
		})
	}
}
