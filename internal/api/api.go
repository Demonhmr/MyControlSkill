// Пакет api — HTTP-слой сервера.
//
// Наружу отдаётся только то, что клиенту положено видеть: сырые ответы 360°
// ни один обработчик не возвращает.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"mycontrolskill/internal/mail"
	"mycontrolskill/internal/ratelimit"
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
	// TrustProxy разрешает брать адрес клиента из X-Forwarded-For.
	// Включать только когда перед сервисом действительно стоит прокси:
	// иначе заголовок подделывается и ограничение частоты обходится.
	TrustProxy bool
	// AllowRegistration решает, можно ли завести аккаунт по этому адресу.
	// nil означает, что можно любому: так сервис вёл себя до появления
	// списка допущенных.
	AllowRegistration func(email string) bool

	limiters struct {
		once sync.Once
		// loginByEmail не даёт завалить письмами один ящик.
		loginByEmail *ratelimit.Limiter
		// loginByIP не даёт рассылать письма по многим ящикам с одного места.
		loginByIP *ratelimit.Limiter
		// invitesByLeader ограничивает рассылку приглашений одним аккаунтом.
		invitesByLeader *ratelimit.Limiter
	}
}

// Пределы подобраны так, чтобы не мешать человеку: запросить ссылку дважды
// подряд — обычное дело, а десяток писем на один адрес за четверть часа уже
// нет.
const (
	loginPerEmail    = 5
	loginEmailBurst  = 2
	loginEmailWindow = 15 * time.Minute

	loginPerIP    = 20
	loginIPBurst  = 5
	loginIPWindow = time.Hour

	invitesPerLeader = 60
	invitesBurst     = 10
	invitesWindow    = time.Hour
)

func (s *Server) initLimiters() {
	s.limiters.once.Do(func() {
		s.limiters.loginByEmail = ratelimit.New(loginPerEmail, loginEmailWindow, loginEmailBurst)
		s.limiters.loginByIP = ratelimit.New(loginPerIP, loginIPWindow, loginIPBurst)
		s.limiters.invitesByLeader = ratelimit.New(invitesPerLeader, invitesWindow, invitesBurst)
	})
}

// tooManyRequests отвечает отказом и подсказывает, когда пробовать снова.
func (s *Server) tooManyRequests(w http.ResponseWriter, wait time.Duration) {
	seconds := int(wait.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	s.writeError(w, http.StatusTooManyRequests, "слишком часто, попробуйте позже")
}

// Register вешает обработчики на общий маршрутизатор.
func (s *Server) Register(mux *http.ServeMux) {
	s.initLimiters()

	mux.HandleFunc("POST /api/auth/login", s.handleLoginRequest)
	mux.HandleFunc("GET /api/auth/callback", s.handleLoginCallback)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/me", s.requireLeader(s.handleMe))

	mux.HandleFunc("POST /api/assessments", s.requireLeader(s.handleCreateAssessment))
	mux.HandleFunc("GET /api/assessments", s.requireLeader(s.handleListAssessments))
	mux.HandleFunc("GET /api/assessments/{id}", s.requireLeader(s.handleGetAssessment))
	mux.HandleFunc("POST /api/assessments/{id}/close", s.requireLeader(s.handleCloseAssessment))
	mux.HandleFunc("POST /api/assessments/{id}/invites", s.requireLeader(s.handleCreateInvite))
	mux.HandleFunc("GET /api/assessments/{id}/profile", s.requireLeader(s.handleProfile))

	mux.HandleFunc("GET /api/state", s.requireLeader(s.handleGetState))
	mux.HandleFunc("PUT /api/state", s.requireLeader(s.handlePutState))
	mux.HandleFunc("POST /api/reflections", s.requireLeader(s.handleCreateReflection))

	mux.HandleFunc("POST /api/hr/org", s.requireLeader(s.handleCreateOrg))
	mux.HandleFunc("POST /api/hr/members", s.requireLeader(s.handleAddMember))
	mux.HandleFunc("GET /api/hr/overview", s.requireLeader(s.handleHROverview))

	mux.HandleFunc("GET /api/me/org", s.requireLeader(s.handleGetMembership))
	mux.HandleFunc("PUT /api/me/org/consent", s.requireLeader(s.handleSetConsent))

	// Анкета респондента — без авторизации: аккаунта у него нет, вся
	// аутентификация это знание токена из ссылки.
	mux.HandleFunc("GET /api/survey/{token}", s.handleGetSurvey)
	mux.HandleFunc("POST /api/survey/{token}", s.handleSubmitSurvey)
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
