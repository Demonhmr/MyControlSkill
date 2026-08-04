package api

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"mycontrolskill/internal/domain"
	"mycontrolskill/internal/scoring"
	"mycontrolskill/internal/store"
)

// surveyPath — путь к анкете респондента. Экран по нему появится следующим
// шагом; ссылка формируется уже сейчас, потому что приглашения без неё
// бессмысленны.
const surveyPath = "/s/"

type assessmentView struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	CreatedAt time.Time  `json:"createdAt"`
	ClosedAt  *time.Time `json:"closedAt"`
	Counts    countsView `json:"counts"`
}

type countsView struct {
	External int  `json:"external"`
	Self     int  `json:"self"`
	Required int  `json:"required"`
	Ready    bool `json:"ready"`
}

// inviteView — приглашение без токена: сам токен существует один раз, в
// ответе на создание.
type inviteView struct {
	ID        string     `json:"id"`
	Role      string     `json:"role"`
	Email     string     `json:"email"`
	CreatedAt time.Time  `json:"createdAt"`
	UsedAt    *time.Time `json:"usedAt"`
}

func toCountsView(c store.Counts) countsView {
	return countsView{
		External: c.External,
		Self:     c.Self,
		Required: domain.MinRespondents,
		Ready:    c.Ready(),
	}
}

func toInviteView(i store.Invite) inviteView {
	return inviteView{
		ID:        i.ID,
		Role:      string(i.Role),
		Email:     i.Email,
		CreatedAt: i.CreatedAt,
		UsedAt:    i.UsedAt,
	}
}

type createAssessmentRequest struct {
	Title string `json:"title"`
}

// handleCreateAssessment заводит новый раунд 360°.
func (s *Server) handleCreateAssessment(w http.ResponseWriter, r *http.Request) {
	leader, _ := LeaderFrom(r.Context())

	var req createAssessmentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "не удалось разобрать запрос")
		return
	}

	a, err := s.Store.CreateAssessment(r.Context(), leader.ID, strings.TrimSpace(req.Title))
	if err != nil {
		s.Log.Error("не удалось создать раунд", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось создать раунд")
		return
	}

	s.writeJSON(w, http.StatusCreated, s.assessmentView(r, a))
}

// handleListAssessments перечисляет раунды текущего руководителя.
func (s *Server) handleListAssessments(w http.ResponseWriter, r *http.Request) {
	leader, _ := LeaderFrom(r.Context())

	list, err := s.Store.AssessmentsByLeader(r.Context(), leader.ID)
	if err != nil {
		s.Log.Error("не удалось прочитать раунды", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось прочитать раунды")
		return
	}

	out := make([]assessmentView, 0, len(list))
	for _, a := range list {
		out = append(out, s.assessmentView(r, a))
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"assessments": out})
}

// handleGetAssessment отдаёт раунд вместе со списком приглашений.
func (s *Server) handleGetAssessment(w http.ResponseWriter, r *http.Request) {
	a, ok := s.ownedAssessment(w, r)
	if !ok {
		return
	}

	invites, err := s.Store.InvitesByAssessment(r.Context(), a.ID)
	if err != nil {
		s.Log.Error("не удалось прочитать приглашения", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось прочитать приглашения")
		return
	}

	views := make([]inviteView, 0, len(invites))
	for _, i := range invites {
		views = append(views, toInviteView(i))
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"assessment": s.assessmentView(r, a),
		"invites":    views,
	})
}

// handleCloseAssessment закрывает раунд: ссылки перестают принимать анкеты.
func (s *Server) handleCloseAssessment(w http.ResponseWriter, r *http.Request) {
	a, ok := s.ownedAssessment(w, r)
	if !ok {
		return
	}

	if err := s.Store.CloseAssessment(r.Context(), a.ID); err != nil {
		s.Log.Error("не удалось закрыть раунд", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось закрыть раунд")
		return
	}

	updated, err := s.Store.AssessmentByID(r.Context(), a.ID)
	if err != nil {
		s.Log.Error("не удалось перечитать раунд", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось перечитать раунд")
		return
	}
	s.writeJSON(w, http.StatusOK, s.assessmentView(r, updated))
}

type createInviteRequest struct {
	Role  string `json:"role"`
	Email string `json:"email"`
}

// handleCreateInvite выдаёт ссылку респонденту.
//
// Ссылка возвращается в ответе ровно один раз: в базе лежит только хэш
// токена, и повторно её не показать — можно лишь выдать новую.
func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	leader, _ := LeaderFrom(r.Context())
	a, ok := s.ownedAssessment(w, r)
	if !ok {
		return
	}

	var req createInviteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "не удалось разобрать запрос")
		return
	}

	role := domain.Role(strings.TrimSpace(req.Role))
	if !role.Valid() {
		s.writeError(w, http.StatusBadRequest, "неизвестная роль респондента")
		return
	}

	// Почта необязательна: ссылку можно передать и вне почты, тогда
	// отправлять нечего.
	email := strings.TrimSpace(req.Email)
	if email != "" {
		addr, err := mail.ParseAddress(email)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "некорректный адрес почты")
			return
		}
		email = addr.Address
	}

	invite, token, err := s.Store.CreateInvite(r.Context(), a.ID, role, email)
	switch {
	case errors.Is(err, store.ErrAssessmentClosed):
		s.writeError(w, http.StatusConflict, "раунд закрыт, приглашения больше не выдаются")
		return
	case err != nil:
		s.Log.Error("не удалось создать приглашение", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось создать приглашение")
		return
	}

	link := s.linkFor(r, surveyPath+token, nil)
	if email != "" {
		if err := s.Mailer.SendInvite(r.Context(), email, link, leader.Name); err != nil {
			// Приглашение уже создано, ссылка в ответе есть — отдать её
			// полезнее, чем упасть: руководитель передаст вручную.
			s.Log.Error("не удалось отправить приглашение", "err", err)
		}
	}

	s.writeJSON(w, http.StatusCreated, map[string]any{
		"invite": toInviteView(invite),
		"link":   link,
	})
}

// handleProfile отдаёт посчитанный профиль.
//
// Ниже порога respondents — 423 с текущими счётчиками: клиенту нужно
// показать «собрано 2 из 3», но ни одного перцентиля он получить не должен,
// иначе отчёт строился бы на шуме.
func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	a, ok := s.ownedAssessment(w, r)
	if !ok {
		return
	}

	counts, err := s.Store.CountResponses(r.Context(), a.ID)
	if err != nil {
		s.Log.Error("не удалось посчитать анкеты", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось посчитать анкеты")
		return
	}
	if !counts.Ready() {
		s.writeJSON(w, http.StatusLocked, map[string]any{
			"error":  "анкет пока недостаточно для расчёта",
			"counts": toCountsView(counts),
		})
		return
	}

	responses, err := s.Store.ResponsesForScoring(r.Context(), a.ID)
	if err != nil {
		s.Log.Error("не удалось прочитать анкеты", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось прочитать анкеты")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"profile": scoring.Compute(responses),
		"counts":  toCountsView(counts),
	})
}

// ownedAssessment достаёт раунд и проверяет, что он принадлежит текущему
// руководителю.
//
// Чужой раунд отдаёт 404, а не 403: по различию этих ответов перебором
// идентификаторов можно было бы выяснить, какие раунды вообще существуют.
func (s *Server) ownedAssessment(w http.ResponseWriter, r *http.Request) (store.Assessment, bool) {
	leader, _ := LeaderFrom(r.Context())

	a, err := s.Store.AssessmentByID(r.Context(), r.PathValue("id"))
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.writeError(w, http.StatusNotFound, "раунд не найден")
		return store.Assessment{}, false
	case err != nil:
		s.Log.Error("не удалось прочитать раунд", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось прочитать раунд")
		return store.Assessment{}, false
	}

	if a.LeaderID != leader.ID {
		s.writeError(w, http.StatusNotFound, "раунд не найден")
		return store.Assessment{}, false
	}
	return a, true
}

// assessmentView собирает представление раунда вместе со счётчиками анкет.
func (s *Server) assessmentView(r *http.Request, a store.Assessment) assessmentView {
	view := assessmentView{
		ID:        a.ID,
		Title:     a.Title,
		CreatedAt: a.CreatedAt,
		ClosedAt:  a.ClosedAt,
	}

	counts, err := s.Store.CountResponses(r.Context(), a.ID)
	if err != nil {
		// Счётчики — справочная часть ответа; из-за них терять сам раунд
		// незачем, но знать о сбое надо.
		s.Log.Error("не удалось посчитать анкеты", "assessment", a.ID, "err", err)
	}
	view.Counts = toCountsView(counts)
	return view
}
