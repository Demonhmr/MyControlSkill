package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestСостояниеПустоеУНовогоАккаунта(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")

	resp, body := doJSON(t, client, http.MethodGet, ts.URL+"/api/state", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("статус %d, ожидался 200", resp.StatusCode)
	}
	raw, _ := json.Marshal(body["state"])
	if string(raw) != "{}" {
		t.Errorf("состояние нового аккаунта = %s", raw)
	}
	if refs, _ := body["reflections"].([]any); len(refs) != 0 {
		t.Errorf("записей у нового аккаунта: %d", len(refs))
	}
}

func TestСостояниеСохраняетсяИЧитается(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")

	payload := `{"state":{"growthPoint":"COM","destructorAcknowledged":true,"interest":{"COM":true}}}`
	resp, _ := doJSON(t, client, http.MethodPut, ts.URL+"/api/state", payload)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("сохранение вернуло %d, ожидался 204", resp.StatusCode)
	}

	_, body := doJSON(t, client, http.MethodGet, ts.URL+"/api/state", "")
	state, _ := body["state"].(map[string]any)
	if state["growthPoint"] != "COM" {
		t.Errorf("точка роста = %v", state["growthPoint"])
	}
	if state["destructorAcknowledged"] != true {
		t.Errorf("отметка деструктора = %v", state["destructorAcknowledged"])
	}

	// Перезапись заменяет состояние целиком, а не сливает его с прежним.
	doJSON(t, client, http.MethodPut, ts.URL+"/api/state", `{"state":{"growthPoint":"REL"}}`)
	_, body = doJSON(t, client, http.MethodGet, ts.URL+"/api/state", "")
	state, _ = body["state"].(map[string]any)
	if state["growthPoint"] != "REL" {
		t.Errorf("точка роста после перезаписи = %v", state["growthPoint"])
	}
	if _, stale := state["destructorAcknowledged"]; stale {
		t.Error("прежнее поле пережило перезапись")
	}
}

func TestСостояниеНеПротекаетМеждуАккаунтами(t *testing.T) {
	ts, mailer, _ := newTestServer(t)

	first := login(t, ts, mailer, "one@example.com")
	doJSON(t, first, http.MethodPut, ts.URL+"/api/state", `{"state":{"growthPoint":"COM"}}`)

	second := login(t, ts, mailer, "two@example.com")
	_, body := doJSON(t, second, http.MethodGet, ts.URL+"/api/state", "")
	raw, _ := json.Marshal(body["state"])
	if string(raw) != "{}" {
		t.Errorf("чужое состояние видно второму аккаунту: %s", raw)
	}
}

func TestНекорректноеСостояниеОтвергается(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")

	cases := map[string]struct {
		payload string
		status  int
	}{
		"пустое тело":     {`{}`, http.StatusBadRequest},
		"битый JSON":      {`{не json`, http.StatusBadRequest},
		"лишнее поле":     {`{"state":{},"whatever":1}`, http.StatusBadRequest},
		"слишком большое": {`{"state":{"x":"` + strings.Repeat("я", MaxStateBytes) + `"}}`, http.StatusRequestEntityTooLarge},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			resp, _ := doJSON(t, client, http.MethodPut, ts.URL+"/api/state", c.payload)
			if resp.StatusCode != c.status {
				t.Errorf("статус %d, ожидался %d", resp.StatusCode, c.status)
			}
		})
	}
}

func TestЗаписиТренажёраСохраняются(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")

	resp, body := doJSON(t, client, http.MethodPost, ts.URL+"/api/reflections",
		`{"code":"COM","text":"  Провёл трудный разговор без смягчения.  "}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("статус %d, ожидался 201", resp.StatusCode)
	}
	if body["text"] != "Провёл трудный разговор без смягчения." {
		t.Errorf("текст не обрезан по краям: %q", body["text"])
	}
	if body["createdAt"] == nil {
		t.Error("в ответе нет момента создания")
	}

	_, state := doJSON(t, client, http.MethodGet, ts.URL+"/api/state", "")
	refs, _ := state["reflections"].([]any)
	if len(refs) != 1 {
		t.Fatalf("записей в состоянии: %d", len(refs))
	}
	first, _ := refs[0].(map[string]any)
	if first["code"] != "COM" {
		t.Errorf("код записи = %v", first["code"])
	}
}

func TestНекорректнаяЗаписьОтвергается(t *testing.T) {
	ts, mailer, _ := newTestServer(t)
	client := login(t, ts, mailer, "lead@example.com")

	bad := map[string]string{
		"нелатинский код": `{"code":"НЕТ","text":"текст"}`,
		"пустой код":      `{"code":"","text":"текст"}`,
		"код с пробелом":  `{"code":"COM VIS","text":"текст"}`,
		"длинный код":     `{"code":"` + strings.Repeat("A", MaxReflectionCodeLength+1) + `","text":"текст"}`,
		"пустой текст":    `{"code":"COM","text":"   "}`,
		"длинный текст":   `{"code":"COM","text":"` + strings.Repeat("я", MaxReflectionLength+1) + `"}`,
	}
	for name, payload := range bad {
		t.Run(name, func(t *testing.T) {
			resp, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/reflections", payload)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("статус %d, ожидался 400", resp.StatusCode)
			}
		})
	}

	// Тренажёр помечает записи не только кодами компетенций: у сценария по
	// критической зоне свой ключ, и словарём компетенций его не проверить.
	good := map[string]string{
		"код деструктора": `{"code":"d3","text":"текст"}`,
		"ключ сценария":   `{"code":"DESTRUCTOR_VIS","text":"текст"}`,
	}
	for name, payload := range good {
		t.Run(name, func(t *testing.T) {
			resp, _ := doJSON(t, client, http.MethodPost, ts.URL+"/api/reflections", payload)
			if resp.StatusCode != http.StatusCreated {
				t.Errorf("статус %d, ожидался 201", resp.StatusCode)
			}
		})
	}
}

func TestСостояниеТребуетВхода(t *testing.T) {
	ts, _, _ := newTestServer(t)
	client := newClient(t)

	paths := []struct{ method, path, body string }{
		{http.MethodGet, "/api/state", ""},
		{http.MethodPut, "/api/state", `{"state":{}}`},
		{http.MethodPost, "/api/reflections", `{"code":"COM","text":"текст"}`},
	}
	for _, p := range paths {
		resp, _ := doJSON(t, client, p.method, ts.URL+p.path, p.body)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: статус %d, ожидался 401", p.method, p.path, resp.StatusCode)
		}
	}
}
