package api

import (
	"errors"
	"net/http"
	"strings"

	"mycontrolskill/internal/store"
)

type deleteAccountRequest struct {
	// ConfirmEmail — свой адрес, введённый вручную. Удаление необратимо, и
	// одного нажатия для него мало.
	ConfirmEmail string `json:"confirmEmail"`
}

// handleDeleteAccount удаляет аккаунт и все данные руководителя.
func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	leader, _ := LeaderFrom(r.Context())

	var req deleteAccountRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "не удалось разобрать запрос")
		return
	}

	// Подтверждение проверяется на сервере, а не только в интерфейсе:
	// операция необратима, и восстановить данные можно будет лишь из копии
	// базы, которая делается раз в сутки.
	if !strings.EqualFold(strings.TrimSpace(req.ConfirmEmail), leader.Email) {
		s.writeError(w, http.StatusBadRequest, "для удаления введите свой адрес почты")
		return
	}

	if err := s.Store.DeleteLeader(r.Context(), leader.ID); err != nil {
		s.Log.Error("не удалось удалить аккаунт", "leader", leader.ID, "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось удалить аккаунт")
		return
	}
	s.Log.Info("аккаунт удалён по запросу владельца", "leader", leader.ID)

	// Сессия вместе с аккаунтом уже исчезла, но cookie в браузере осталась —
	// гасим, чтобы он не таскал мёртвый идентификатор.
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

// handleDeleteAssessment удаляет раунд вместе с собранными по нему анкетами.
func (s *Server) handleDeleteAssessment(w http.ResponseWriter, r *http.Request) {
	a, ok := s.ownedAssessment(w, r)
	if !ok {
		return
	}

	if err := s.Store.DeleteAssessment(r.Context(), a.ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.writeError(w, http.StatusNotFound, "раунд не найден")
			return
		}
		s.Log.Error("не удалось удалить раунд", "assessment", a.ID, "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось удалить раунд")
		return
	}

	s.Log.Info("раунд удалён", "assessment", a.ID)
	s.writeJSON(w, http.StatusNoContent, nil)
}
