package api

import (
	"errors"
	"net/http"
	"time"

	"mycontrolskill/internal/store"
)

// membershipView — что руководитель знает о своём участии в организации.
type membershipView struct {
	Org  orgView `json:"org"`
	Role string  `json:"role"`
	// ConsentGranted — разрешён ли показ профиля HR-службе.
	ConsentGranted bool `json:"consentGranted"`
	// ConsentAt — когда согласие было дано. nil, если согласия нет.
	ConsentAt *time.Time `json:"consentAt"`
}

type consentRequest struct {
	Granted bool `json:"granted"`
}

// handleGetMembership отдаёт участие текущего руководителя в организации.
//
// Отдельно от /api/me: аккаунт есть у всех, а организация — не у всех, и
// смешивать «нет организации» с «нет аккаунта» в одном ответе неудобно.
func (s *Server) handleGetMembership(w http.ResponseWriter, r *http.Request) {
	leader, _ := LeaderFrom(r.Context())

	org, role, err := s.Store.OrgForLeader(r.Context(), leader.ID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.writeError(w, http.StatusNotFound, "вы не состоите в организации")
		return
	case err != nil:
		s.Log.Error("не удалось прочитать организацию", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось прочитать организацию")
		return
	}

	member, err := s.Store.MemberOf(r.Context(), org.ID, leader.ID)
	if err != nil {
		s.Log.Error("не удалось прочитать участие", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось прочитать участие")
		return
	}

	s.writeJSON(w, http.StatusOK, membershipView{
		Org:            orgView{ID: org.ID, Name: org.Name},
		Role:           string(role),
		ConsentGranted: member.ProfileConsentGranted(),
		ConsentAt:      member.ProfileConsentAt,
	})
}

// handleSetConsent выдаёт или отзывает согласие на показ профиля HR-службе.
//
// Решение принимает только сам человек: эйчар добавляет его в состав, но
// разрешить показывать свои числа за него не может.
func (s *Server) handleSetConsent(w http.ResponseWriter, r *http.Request) {
	leader, _ := LeaderFrom(r.Context())

	var req consentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "не удалось разобрать запрос")
		return
	}

	member, err := s.Store.SetProfileConsent(r.Context(), leader.ID, req.Granted)
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.writeError(w, http.StatusNotFound, "вы не состоите в организации")
		return
	case err != nil:
		s.Log.Error("не удалось сохранить согласие", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось сохранить согласие")
		return
	}

	s.Log.Info("согласие на показ профиля изменено", "leader", leader.ID, "granted", req.Granted)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"consentGranted": member.ProfileConsentGranted(),
		"consentAt":      member.ProfileConsentAt,
	})
}
