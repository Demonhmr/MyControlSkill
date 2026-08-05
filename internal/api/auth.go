package api

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	"strings"

	"mycontrolskill/internal/store"
)

// sessionCookie — имя cookie с идентификатором сессии.
const sessionCookie = "mcs_session"

type leaderKey struct{}

// LeaderFrom достаёт руководителя, положенного в контекст middleware.
func LeaderFrom(ctx context.Context) (store.Leader, bool) {
	l, ok := ctx.Value(leaderKey{}).(store.Leader)
	return l, ok
}

type loginRequest struct {
	Email string `json:"email"`
}

// handleLoginRequest принимает почту и отправляет ссылку для входа.
//
// Ответ одинаковый в любом случае: по нему нельзя узнать, есть ли такой
// аккаунт. Правда, первый вход аккаунт и создаёт, так что скрывать пока
// особо нечего — но уносить эту утечку в интерфейс не стоит.
func (s *Server) handleLoginRequest(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "не удалось разобрать запрос")
		return
	}

	addr, err := mail.ParseAddress(strings.TrimSpace(req.Email))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "некорректный адрес почты")
		return
	}

	email := strings.ToLower(addr.Address)

	// Отказ по списку допущенных — до всего остального: письмо тому, кто
	// всё равно не сможет войти, бессмысленно, а человеку лучше узнать
	// причину сразу, чем после перехода по ссылке.
	//
	// База спрашивается только про адреса вне списка: у кого аккаунт уже
	// есть, тот входит всегда — иначе подрядчик, которого эйчар добавил с
	// почтой на чужом домене, не смог бы даже запросить ссылку.
	//
	// Цена такой проверки — по разнице между 202 и 403 для адреса вне
	// списка можно узнать, что у него здесь есть аккаунт. Обмен осознанный:
	// сломанный вход для добавленных эйчаром людей хуже, чем эта утечка.
	if !s.allowRegistration(email) && !s.hasAccount(r, email) {
		s.Log.Info("запрос ссылки отклонён списком допущенных", "email", email)
		s.writeError(w, http.StatusForbidden, "этот адрес не допущен к сервису")
		return
	}

	// Два разных ограничения. По адресу — чтобы нельзя было завалить
	// письмами один ящик; по источнику запроса — чтобы нельзя было
	// разослать письма по многим ящикам, меняя адрес в каждом запросе.
	if ok, wait := s.limiters.loginByEmail.Allow(email); !ok {
		s.Log.Info("запрос ссылки ограничен по адресу", "email", email)
		s.tooManyRequests(w, wait)
		return
	}
	if ok, wait := s.limiters.loginByIP.Allow(s.clientIP(r)); !ok {
		s.Log.Warn("запрос ссылки ограничен по источнику", "ip", s.clientIP(r))
		s.tooManyRequests(w, wait)
		return
	}

	token, err := s.Store.CreateLoginToken(r.Context(), addr.Address)
	if err != nil {
		s.Log.Error("не удалось создать ссылку для входа", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось отправить ссылку")
		return
	}

	link := s.linkFor(r, "/api/auth/callback", url.Values{"token": {token}})
	if err := s.Mailer.SendLoginLink(r.Context(), addr.Address, link); err != nil {
		s.Log.Error("не удалось отправить ссылку для входа", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось отправить ссылку")
		return
	}

	// Мусор чистится здесь: отдельный планировщик ради двух DELETE не нужен,
	// а входы происходят достаточно часто.
	if err := s.Store.PurgeExpiredAuth(r.Context()); err != nil {
		s.Log.Warn("не удалось почистить протухшие токены", "err", err)
	}

	s.writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "ссылка отправлена, если адрес указан верно",
	})
}

// handleLoginCallback гасит ссылку, заводит сессию и отправляет в приложение.
//
// Это переход по ссылке из письма, поэтому ответ — редирект, а не JSON.
func (s *Server) handleLoginCallback(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		s.redirectWithError(w, r, "no-token")
		return
	}

	leader, err := s.Store.ConsumeLoginToken(r.Context(), token, s.allowRegistration)
	switch {
	case errors.Is(err, store.ErrRegistrationClosed):
		s.redirectWithError(w, r, "not-allowed")
		return
	case errors.Is(err, store.ErrTokenExpired):
		s.redirectWithError(w, r, "expired")
		return
	case errors.Is(err, store.ErrTokenUsed):
		s.redirectWithError(w, r, "used")
		return
	case errors.Is(err, store.ErrNotFound):
		s.redirectWithError(w, r, "invalid")
		return
	case err != nil:
		s.Log.Error("не удалось выполнить вход", "err", err)
		s.redirectWithError(w, r, "server")
		return
	}

	sessionToken, session, err := s.Store.CreateSession(r.Context(), leader.ID)
	if err != nil {
		s.Log.Error("не удалось создать сессию", "err", err)
		s.redirectWithError(w, r, "server")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:  sessionCookie,
		Value: sessionToken,
		Path:  "/",
		// HttpOnly: сессия недоступна из JS, поэтому XSS её не украдёт.
		HttpOnly: true,
		Secure:   s.SecureCookies,
		// Lax, а не Strict: переход из почтового клиента — межсайтовый,
		// и при Strict cookie не доехала бы до приложения.
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	})

	s.Log.Info("вход выполнен", "leader", leader.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleLogout завершает сессию.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.Store.DeleteSession(r.Context(), c.Value); err != nil {
			s.Log.Error("не удалось завершить сессию", "err", err)
		}
	}

	// Cookie гасится в любом случае: если сессии в базе уже нет, браузер всё
	// равно не должен таскать мёртвый идентификатор.
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	s.writeJSON(w, http.StatusNoContent, nil)
}

// handleMe отдаёт текущего руководителя.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	leader, _ := LeaderFrom(r.Context())
	s.writeJSON(w, http.StatusOK, map[string]any{
		"id":        leader.ID,
		"email":     leader.Email,
		"name":      leader.Name,
		"createdAt": leader.CreatedAt,
	})
}

// requireLeader пускает дальше только с действующей сессией.
func (s *Server) requireLeader(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			s.writeError(w, http.StatusUnauthorized, "требуется вход")
			return
		}

		leader, err := s.Store.LeaderBySession(r.Context(), c.Value)
		switch {
		case errors.Is(err, store.ErrNotFound), errors.Is(err, store.ErrTokenExpired):
			s.writeError(w, http.StatusUnauthorized, "требуется вход")
			return
		case err != nil:
			s.Log.Error("не удалось проверить сессию", "err", err)
			s.writeError(w, http.StatusInternalServerError, "внутренняя ошибка")
			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), leaderKey{}, leader)))
	}
}

// hasAccount сообщает, заведён ли уже аккаунт по этому адресу.
func (s *Server) hasAccount(r *http.Request, email string) bool {
	_, err := s.Store.LeaderByEmail(r.Context(), email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.Log.Error("не удалось проверить аккаунт", "err", err)
	}
	return err == nil
}

// allowRegistration применяет список допущенных. Без списка разрешено всем.
func (s *Server) allowRegistration(email string) bool {
	if s.AllowRegistration == nil {
		return true
	}
	return s.AllowRegistration(email)
}

// linkFor собирает абсолютную ссылку для письма.
func (s *Server) linkFor(r *http.Request, path string, query url.Values) string {
	base := s.BaseURL
	if base == "" {
		// Резерв для разработки. В проде за обратным прокси заголовкам
		// доверять нельзя, поэтому там задаётся MCS_BASE_URL.
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}

	link := strings.TrimSuffix(base, "/") + path
	if len(query) > 0 {
		link += "?" + query.Encode()
	}
	return link
}

// redirectWithError возвращает пользователя в приложение с пометкой о
// причине отказа: разбираться с ней должен интерфейс, а не голый HTTP-код.
func (s *Server) redirectWithError(w http.ResponseWriter, r *http.Request, reason string) {
	http.Redirect(w, r, "/?login_error="+url.QueryEscape(reason), http.StatusSeeOther)
}
