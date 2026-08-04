package api

import (
	"errors"
	"net/http"
	"strings"

	"mycontrolskill/internal/domain"
	"mycontrolskill/internal/store"
)

// surveyView — то, что видит респондент, открыв ссылку.
//
// Текстов анкеты здесь нет: их рисует клиент из собственных данных.
// Дублировать девятнадцать компетенций с формулировками ещё и на сервере
// незачем — сервер проверяет коды, а не тексты.
type surveyView struct {
	// Role назначена руководителем при приглашении и подмене не подлежит.
	Role string `json:"role"`
	// LeaderName — кого оценивают. Пусто быть не должно: если имя не
	// заполнено, подставляется почта, иначе респондент не понимает, о ком речь.
	LeaderName string `json:"leaderName"`
	// Used — по ссылке уже отвечали.
	Used bool `json:"used"`
	// Closed — раунд закрыт, анкеты больше не принимаются.
	Closed bool `json:"closed"`
}

type answerInput struct {
	Kind      string `json:"kind"`
	Code      string `json:"code"`
	ItemIndex int    `json:"itemIndex"`
	// Value равно null, когда респондент выбрал «не могу оценить».
	Value *int `json:"value"`
}

type openAnswerInput struct {
	QuestionIndex int    `json:"questionIndex"`
	Text          string `json:"text"`
}

type submitSurveyRequest struct {
	Tenure      string            `json:"tenure"`
	Answers     []answerInput     `json:"answers"`
	OpenAnswers []openAnswerInput `json:"openAnswers"`
}

// handleGetSurvey отдаёт контекст анкеты по ссылке-приглашению.
//
// Без авторизации: у респондента нет и не будет аккаунта, вся его
// аутентификация — это знание токена из ссылки.
func (s *Server) handleGetSurvey(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")

	invite, err := s.Store.InviteByToken(r.Context(), token)
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.writeError(w, http.StatusNotFound, "ссылка недействительна")
		return
	case err != nil:
		s.Log.Error("не удалось прочитать приглашение", "err", err)
		s.writeError(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}

	assessment, err := s.Store.AssessmentByID(r.Context(), invite.AssessmentID)
	if err != nil {
		s.Log.Error("не удалось прочитать раунд", "err", err)
		s.writeError(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}

	leader, err := s.Store.LeaderByAssessment(r.Context(), invite.AssessmentID)
	if err != nil {
		s.Log.Error("не удалось прочитать руководителя", "err", err)
		s.writeError(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}

	// Использованная ссылка и закрытый раунд отдают 200 с пометкой, а не
	// ошибку: респонденту нужно объяснение на экране, а не пустая страница.
	s.writeJSON(w, http.StatusOK, surveyView{
		Role:       string(invite.Role),
		LeaderName: leaderDisplayName(leader),
		Used:       invite.Used(),
		Closed:     assessment.Closed(),
	})
}

// handleSubmitSurvey принимает заполненную анкету.
func (s *Server) handleSubmitSurvey(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")

	var req submitSurveyRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "не удалось разобрать анкету")
		return
	}

	sub := domain.Submission{
		Tenure:      domain.Tenure(strings.TrimSpace(req.Tenure)),
		Answers:     make([]domain.Answer, 0, len(req.Answers)),
		OpenAnswers: make([]domain.OpenAnswer, 0, len(req.OpenAnswers)),
	}
	for _, a := range req.Answers {
		sub.Answers = append(sub.Answers, domain.Answer{
			Kind:      domain.Kind(a.Kind),
			Code:      a.Code,
			ItemIndex: a.ItemIndex,
			Value:     a.Value,
		})
	}
	for _, o := range req.OpenAnswers {
		text := strings.TrimSpace(o.Text)
		if text == "" {
			// Пустой ответ не хранится: пустая строка и пропущенный вопрос
			// значат одно и то же.
			continue
		}
		sub.OpenAnswers = append(sub.OpenAnswers, domain.OpenAnswer{
			QuestionIndex: o.QuestionIndex,
			Text:          text,
		})
	}

	// Роль сюда не приходит вовсе: её подставит хранилище из приглашения.
	_, err := s.Store.SubmitByToken(r.Context(), token, sub)
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.writeError(w, http.StatusNotFound, "ссылка недействительна")
		return
	case errors.Is(err, store.ErrInviteUsed):
		s.writeError(w, http.StatusConflict, "по этой ссылке анкета уже отправлена")
		return
	case errors.Is(err, store.ErrAssessmentClosed):
		s.writeError(w, http.StatusConflict, "сбор ответов по этому раунду завершён")
		return
	case errors.Is(err, store.ErrInvalidSubmission):
		s.Log.Info("анкета отклонена", "err", err)
		s.writeError(w, http.StatusBadRequest, "анкета не прошла проверку")
		return
	case err != nil:
		s.Log.Error("не удалось сохранить анкету", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось сохранить анкету")
		return
	}

	s.writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// leaderDisplayName выбирает, как назвать оцениваемого.
//
// Имя пока нигде не задаётся, поэтому почта — рабочий запасной вариант:
// респондент всё равно знает, кого его позвали оценить.
func leaderDisplayName(l store.Leader) string {
	if name := strings.TrimSpace(l.Name); name != "" {
		return name
	}
	return l.Email
}
