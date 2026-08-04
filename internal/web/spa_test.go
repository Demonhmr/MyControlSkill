package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func testFS() fs.FS {
	return fstest.MapFS{
		"index.html":            {Data: []byte("<html>оболочка</html>")},
		"assets/app-a1b2c3.js":  {Data: []byte("console.log(1)")},
		"assets/app-a1b2c3.css": {Data: []byte("body{}")},
	}
}

func get(t *testing.T, h http.Handler, target string) *http.Response {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec.Result()
}

func TestОтдаётФайлПоТочномуПути(t *testing.T) {
	resp := get(t, SPA(testFS()), "/assets/app-a1b2c3.js")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("статус = %d, ожидался 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control для ассета = %q", got)
	}
}

func TestНеизвестныйПутьОтдаётОболочку(t *testing.T) {
	// Клиентская навигация: такого файла в сборке нет, но 404 отдавать нельзя.
	for _, target := range []string{"/plan", "/s/abc123", "/assets", "/глубоко/вложенный/путь"} {
		resp := get(t, SPA(testFS()), target)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: статус = %d, ожидался 200", target, resp.StatusCode)
		}
		if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
			t.Errorf("%s: Cache-Control = %q, ожидался no-cache", target, got)
		}
	}
}

func TestКореньОтдаётIndex(t *testing.T) {
	resp := get(t, SPA(testFS()), "/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("статус = %d, ожидался 200", resp.StatusCode)
	}
}

func TestВыходЗаПределыКаталогаНеРаботает(t *testing.T) {
	// path.Clean схлопывает ../, поэтому запрос попадает в оболочку,
	// а не к файлам вне сборки.
	resp := get(t, SPA(testFS()), "/../../etc/passwd")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("статус = %d, ожидался 200 (оболочка)", resp.StatusCode)
	}
}

func TestБезIndexОтдаётся500(t *testing.T) {
	empty := fstest.MapFS{}
	resp := get(t, SPA(empty), "/")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("статус = %d, ожидался 500", resp.StatusCode)
	}
}
