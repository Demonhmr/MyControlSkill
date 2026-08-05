package api

import (
	"errors"
	"net/http"
	"net/mail"
	"sort"
	"strings"

	"mycontrolskill/internal/domain"
	"mycontrolskill/internal/scoring"
	"mycontrolskill/internal/store"
)

// StrengthThreshold — перцентиль, с которого компетенция считается сильной
// стороной. Ниже него развитие почти не меняет восприятие руководителя.
const StrengthThreshold = 70

// CriticalThreshold — перцентиль, ниже которого зона считается критической
// и обнуляет эффект сильных сторон.
const CriticalThreshold = 10

// TopStrengths — сколько сильных сторон показывать в сводке.
const TopStrengths = 2

type orgView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type codeScore struct {
	Code       string `json:"code"`
	Percentile int    `json:"percentile"`
}

// hrLeaderView — строка сводки по одному руководителю.
//
// Ни сырых ответов, ни открытых комментариев здесь нет: эйчар видит
// агрегаты, как и сам руководитель.
type hrLeaderView struct {
	LeaderID string `json:"leaderId"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	// Ready — можно ли показывать числа: набрались анкеты и есть согласие.
	Ready bool `json:"ready"`
	// ConsentGranted — разрешил ли человек показывать свой профиль HR.
	// Без согласия чисел нет, сколько бы анкет ни собралось.
	ConsentGranted bool `json:"consentGranted"`
	// Counts заполнен всегда: он объясняет, почему чисел нет.
	Counts      *countsView `json:"counts"`
	Destructors []codeScore `json:"destructors"`
	Strengths   []codeScore `json:"strengths"`
	HasCritical bool        `json:"hasCritical"`
}

type createOrgRequest struct {
	Name string `json:"name"`
}

type addMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// handleCreateOrg заводит организацию; создатель становится эйчаром.
func (s *Server) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	leader, _ := LeaderFrom(r.Context())

	var req createOrgRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "не удалось разобрать запрос")
		return
	}

	org, err := s.Store.CreateOrg(r.Context(), req.Name, leader.ID)
	switch {
	case errors.Is(err, store.ErrAlreadyInOrg):
		s.writeError(w, http.StatusConflict, "вы уже состоите в организации")
		return
	case err != nil:
		if strings.Contains(err.Error(), "название") {
			s.writeError(w, http.StatusBadRequest, "укажите название организации")
			return
		}
		s.Log.Error("не удалось создать организацию", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось создать организацию")
		return
	}

	s.writeJSON(w, http.StatusCreated, orgView{ID: org.ID, Name: org.Name})
}

// handleAddMember добавляет руководителя в организацию по адресу почты.
func (s *Server) handleAddMember(w http.ResponseWriter, r *http.Request) {
	org, _, ok := s.requireHR(w, r)
	if !ok {
		return
	}

	var req addMemberRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "не удалось разобрать запрос")
		return
	}

	addr, err := mail.ParseAddress(strings.TrimSpace(req.Email))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "некорректный адрес почты")
		return
	}

	role := store.OrgRole(strings.TrimSpace(req.Role))
	if role == "" {
		role = store.OrgRoleLeader
	}
	if !role.Valid() {
		s.writeError(w, http.StatusBadRequest, "неизвестная роль участника")
		return
	}

	member, err := s.Store.AddOrgMember(r.Context(), org.ID, addr.Address, role)
	switch {
	case errors.Is(err, store.ErrAlreadyInOrg):
		s.writeError(w, http.StatusConflict, "этот человек уже состоит в другой организации")
		return
	case err != nil:
		s.Log.Error("не удалось добавить участника", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось добавить участника")
		return
	}

	s.writeJSON(w, http.StatusCreated, map[string]any{
		"leaderId": member.LeaderID,
		"email":    member.Email,
		"role":     string(member.Role),
	})
}

// handleHROverview собирает сводку по организации.
//
// По каждому участнику берётся его последний раунд. Порог респондентов
// действует и здесь: пока анкет мало, эйчар видит счётчики, а не числа —
// иначе организационные решения принимались бы по шуму.
func (s *Server) handleHROverview(w http.ResponseWriter, r *http.Request) {
	org, _, ok := s.requireHR(w, r)
	if !ok {
		return
	}

	members, err := s.Store.OrgMembers(r.Context(), org.ID)
	if err != nil {
		s.Log.Error("не удалось прочитать состав", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось прочитать состав")
		return
	}

	viewer, _ := LeaderFrom(r.Context())

	views := make([]hrLeaderView, 0, len(members))
	for _, m := range members {
		view, err := s.leaderOverview(r, m, viewer.ID)
		if err != nil {
			s.Log.Error("не удалось собрать сводку по участнику", "leader", m.LeaderID, "err", err)
			s.writeError(w, http.StatusInternalServerError, "не удалось собрать сводку")
			return
		}
		views = append(views, view)
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"org":     orgView{ID: org.ID, Name: org.Name},
		"leaders": views,
	})
}

func (s *Server) leaderOverview(r *http.Request, m store.Member, viewerID string) (hrLeaderView, error) {
	view := hrLeaderView{
		LeaderID:    m.LeaderID,
		Name:        displayName(m),
		Email:       m.Email,
		Role:        string(m.Role),
		Destructors: []codeScore{},
		Strengths:   []codeScore{},
	}

	// Свои данные человек видит всегда: согласие нужно на показ другим, а
	// не самому себе.
	view.ConsentGranted = m.ProfileConsentGranted() || m.LeaderID == viewerID
	if !view.ConsentGranted {
		// Счётчики тоже не показываем: сколько анкет собрал человек — уже
		// сведения о нём, и без согласия их выдавать не за что.
		return view, nil
	}

	assessment, err := s.Store.LatestAssessment(r.Context(), m.LeaderID)
	if errors.Is(err, store.ErrNotFound) {
		// Раундов ещё нет — так и скажем, нулями это подменять нельзя.
		view.Counts = &countsView{Required: countsRequired()}
		return view, nil
	}
	if err != nil {
		return hrLeaderView{}, err
	}

	counts, err := s.Store.CountResponses(r.Context(), assessment.ID)
	if err != nil {
		return hrLeaderView{}, err
	}
	c := toCountsView(counts)
	view.Counts = &c

	if !counts.Ready() {
		return view, nil
	}

	responses, err := s.Store.ResponsesForScoring(r.Context(), assessment.ID)
	if err != nil {
		return hrLeaderView{}, err
	}
	profile := scoring.Compute(responses)

	view.Ready = true
	for _, d := range profile.Destructors {
		if d.Percentile == nil {
			continue
		}
		view.Destructors = append(view.Destructors, codeScore{Code: d.Code, Percentile: *d.Percentile})
		if *d.Percentile < CriticalThreshold {
			view.HasCritical = true
		}
	}
	view.Strengths = topStrengths(profile.Competencies)
	return view, nil
}

// topStrengths отбирает самые сильные компетенции.
func topStrengths(competencies []scoring.Score) []codeScore {
	strong := make([]codeScore, 0, len(competencies))
	for _, c := range competencies {
		if c.Percentile != nil && *c.Percentile >= StrengthThreshold {
			strong = append(strong, codeScore{Code: c.Code, Percentile: *c.Percentile})
		}
	}
	sort.SliceStable(strong, func(i, j int) bool { return strong[i].Percentile > strong[j].Percentile })
	if len(strong) > TopStrengths {
		strong = strong[:TopStrengths]
	}
	return strong
}

// requireHR пускает дальше только эйчара организации.
func (s *Server) requireHR(w http.ResponseWriter, r *http.Request) (store.Org, store.OrgRole, bool) {
	leader, _ := LeaderFrom(r.Context())

	org, role, err := s.Store.OrgForLeader(r.Context(), leader.ID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.writeError(w, http.StatusNotFound, "вы не состоите в организации")
		return store.Org{}, "", false
	case err != nil:
		s.Log.Error("не удалось прочитать организацию", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось прочитать организацию")
		return store.Org{}, "", false
	}

	if role != store.OrgRoleHR {
		// Здесь именно 403, а не 404: своё участие в организации человек и
		// так видит, скрывать нечего — не хватает только прав.
		s.writeError(w, http.StatusForbidden, "сводка доступна только роли HR")
		return store.Org{}, "", false
	}
	return org, role, true
}

// countsRequired — порог респондентов для показа в сводке.
func countsRequired() int { return domain.MinRespondents }

func displayName(m store.Member) string {
	if name := strings.TrimSpace(m.Name); name != "" {
		return name
	}
	return m.Email
}
