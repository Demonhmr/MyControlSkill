// Пакет api — HTTP-слой сервера.
//
// Наружу отдаётся только то, что клиенту положено видеть: сырые ответы 360°
// ни один обработчик не возвращает.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"mycontrolskill/internal/mail"
	"mycontrolskill/internal/store"
)

// Server держит зависимости обработчиков.
type Server struct {
	Store  *store.Store
	Mailer mail.Mailer
	Log    *slog.Logger

	// BaseURL — внешний адрес сервиса для ссылок в письмах. Если пуст,
	// адрес собирается из входящего запроса: в разработке это удобно, в
	// проде за обратным прокси на заголовки полагаться нельзя.
	BaseURL string
	// SecureCookies включает флаг Secure у cookie сессии. Выключается для
	// локальной разработки по http, где браузер такую cookie не сохранит.
	SecureCookies bool
}

// Register вешает обработчики на общий маршрутизатор.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", s.handleLoginRequest)
	mux.HandleFunc("GET /api/auth/callback", s.handleLoginCallback)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/me", s.requireLeader(s.handleMe))
}

// writeJSON отдаёт значение как JSON.
func (s *Server) writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// Ответы API персональны, кэшировать их нельзя ни браузеру, ни прокси.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// Заголовки уже ушли, поправить ответ нечем — остаётся запись в лог.
		s.Log.Error("не удалось записать ответ", "err", err)
	}
}

// writeError отдаёт ошибку в едином виде.
func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message})
}

// decodeJSON читает тело запроса, ограничивая его размер.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
