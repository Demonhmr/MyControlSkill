package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"mycontrolskill/internal/scoring"
	"mycontrolskill/internal/store"
)

// exportInvite — кого позвали в раунд.
//
// Это данные, которые руководитель ввёл сам: он и так видит их на экране
// раунда. Момент ответа тоже — по нему он напоминает неответившим.
type exportInvite struct {
	Role      string     `json:"role"`
	Email     string     `json:"email"`
	CreatedAt time.Time  `json:"createdAt"`
	UsedAt    *time.Time `json:"usedAt"`
}

type exportAssessment struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	CreatedAt time.Time      `json:"createdAt"`
	ClosedAt  *time.Time     `json:"closedAt"`
	Counts    countsView     `json:"counts"`
	Invites   []exportInvite `json:"invites"`
	// Profile заполняется только для посчитанных раундов.
	Profile *scoring.Profile `json:"profile"`
}

type exportReflection struct {
	Code      string    `json:"code"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

type exportMembership struct {
	Org            orgView    `json:"org"`
	Role           string     `json:"role"`
	ConsentGranted bool       `json:"consentGranted"`
	ConsentAt      *time.Time `json:"consentAt"`
}

// exportFile — всё, что сервис хранит о руководителе.
type exportFile struct {
	// Note объясняет получателю, чего в файле нет и почему.
	Note       string    `json:"_note"`
	ExportedAt time.Time `json:"exportedAt"`

	Account struct {
		ID        string    `json:"id"`
		Email     string    `json:"email"`
		Name      string    `json:"name"`
		CreatedAt time.Time `json:"createdAt"`
	} `json:"account"`

	Membership  *exportMembership  `json:"membership"`
	Assessments []exportAssessment `json:"assessments"`
	State       json.RawMessage    `json:"state"`
	Reflections []exportReflection `json:"reflections"`
}

const exportNote = "Выгрузка содержит агрегаты, но не отдельные ответы респондентов: " +
	"по ним восстанавливается, кто именно как ответил, и анонимность 360° на этом заканчивается."

// handleExport отдаёт файл со всеми данными руководителя.
//
// Сырых анкет в нём нет намеренно. Право на выгрузку своих данных не
// распространяется на чужие: ответы коллег — это их обратная связь, и
// руководитель имеет право на её сводный результат, а не на исходники.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	leader, _ := LeaderFrom(r.Context())

	file := exportFile{Note: exportNote, ExportedAt: time.Now().UTC()}
	file.Account.ID = leader.ID
	file.Account.Email = leader.Email
	file.Account.Name = leader.Name
	file.Account.CreatedAt = leader.CreatedAt

	if org, role, err := s.Store.OrgForLeader(r.Context(), leader.ID); err == nil {
		member, err := s.Store.MemberOf(r.Context(), org.ID, leader.ID)
		if err != nil {
			s.Log.Error("не удалось прочитать участие", "err", err)
			s.writeError(w, http.StatusInternalServerError, "не удалось собрать выгрузку")
			return
		}
		file.Membership = &exportMembership{
			Org:            orgView{ID: org.ID, Name: org.Name},
			Role:           string(role),
			ConsentGranted: member.ProfileConsentGranted(),
			ConsentAt:      member.ProfileConsentAt,
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		s.Log.Error("не удалось прочитать организацию", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось собрать выгрузку")
		return
	}

	assessments, err := s.Store.AssessmentsByLeader(r.Context(), leader.ID)
	if err != nil {
		s.Log.Error("не удалось прочитать раунды", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось собрать выгрузку")
		return
	}

	file.Assessments = make([]exportAssessment, 0, len(assessments))
	for _, a := range assessments {
		item, err := s.exportAssessment(r, a)
		if err != nil {
			s.Log.Error("не удалось собрать раунд для выгрузки", "assessment", a.ID, "err", err)
			s.writeError(w, http.StatusInternalServerError, "не удалось собрать выгрузку")
			return
		}
		file.Assessments = append(file.Assessments, item)
	}

	state, err := s.Store.LoadLeaderState(r.Context(), leader.ID)
	if err != nil {
		s.Log.Error("не удалось прочитать состояние", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось собрать выгрузку")
		return
	}
	file.State = state

	reflections, err := s.Store.Reflections(r.Context(), leader.ID, 0)
	if err != nil {
		s.Log.Error("не удалось прочитать записи", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось собрать выгрузку")
		return
	}
	file.Reflections = make([]exportReflection, 0, len(reflections))
	for _, ref := range reflections {
		file.Reflections = append(file.Reflections,
			exportReflection{Code: ref.Code, Text: ref.Text, CreatedAt: ref.CreatedAt})
	}

	// Скачивается файлом, а не открывается вкладкой: это выгрузка, её
	// сохраняют.
	filename := fmt.Sprintf("mycontrolskill-%s.json", time.Now().UTC().Format("2006-01-02"))
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	s.writeJSON(w, http.StatusOK, file)
}

func (s *Server) exportAssessment(r *http.Request, a store.Assessment) (exportAssessment, error) {
	counts, err := s.Store.CountResponses(r.Context(), a.ID)
	if err != nil {
		return exportAssessment{}, err
	}

	item := exportAssessment{
		ID:        a.ID,
		Title:     a.Title,
		CreatedAt: a.CreatedAt,
		ClosedAt:  a.ClosedAt,
		Counts:    toCountsView(counts),
		Invites:   []exportInvite{},
	}

	invites, err := s.Store.InvitesByAssessment(r.Context(), a.ID)
	if err != nil {
		return exportAssessment{}, err
	}
	for _, inv := range invites {
		item.Invites = append(item.Invites, exportInvite{
			Role:      string(inv.Role),
			Email:     inv.Email,
			CreatedAt: inv.CreatedAt,
			UsedAt:    inv.UsedAt,
		})
	}

	if counts.Ready() {
		responses, err := s.Store.ResponsesForScoring(r.Context(), a.ID)
		if err != nil {
			return exportAssessment{}, err
		}
		profile := scoring.Compute(responses)
		item.Profile = &profile
	}
	return item, nil
}
