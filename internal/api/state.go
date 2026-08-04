package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxStateBytes ограничивает размер рабочего состояния.
//
// Состояние — это отметки на экранах: выбранная точка роста, интересы,
// потребности команды. Даже с запасом оно на два порядка меньше предела;
// предел нужен только чтобы в базу нельзя было залить произвольный объём.
const MaxStateBytes = 64 << 10

// MaxReflectionLength — предел длины записи из тренажёра, в символах.
const MaxReflectionLength = 5000

// MaxReflectionCodeLength — предел длины метки записи.
const MaxReflectionCodeLength = 64

func validReflectionCode(code string) bool {
	if code == "" || len(code) > MaxReflectionCodeLength {
		return false
	}
	for _, r := range code {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

type reflectionView struct {
	Code      string    `json:"code"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

type stateRequest struct {
	State json.RawMessage `json:"state"`
}

type createReflectionRequest struct {
	Code string `json:"code"`
	Text string `json:"text"`
}

// handleGetState отдаёт рабочее состояние руководителя вместе с записями
// из тренажёра.
//
// Одним запросом, а не двумя: клиент всё равно не может показать экраны, не
// получив обе части, и ждать их по очереди было бы медленнее без выигрыша.
func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	leader, _ := LeaderFrom(r.Context())

	data, err := s.Store.LoadLeaderState(r.Context(), leader.ID)
	if err != nil {
		s.Log.Error("не удалось прочитать состояние", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось прочитать состояние")
		return
	}

	reflections, err := s.Store.Reflections(r.Context(), leader.ID, 0)
	if err != nil {
		s.Log.Error("не удалось прочитать записи", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось прочитать записи")
		return
	}

	views := make([]reflectionView, 0, len(reflections))
	for _, ref := range reflections {
		views = append(views, reflectionView{Code: ref.Code, Text: ref.Text, CreatedAt: ref.CreatedAt})
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"state":       data,
		"reflections": views,
	})
}

// handlePutState перезаписывает рабочее состояние целиком.
//
// Целиком, а не по полям: это набор отметок одного экрана, редактирует их
// один человек со своего устройства, и слияние изменений здесь ничего не
// решает — только усложняет.
func (s *Server) handlePutState(w http.ResponseWriter, r *http.Request) {
	leader, _ := LeaderFrom(r.Context())

	var req stateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "не удалось разобрать состояние")
		return
	}
	if len(req.State) == 0 {
		s.writeError(w, http.StatusBadRequest, "состояние не передано")
		return
	}
	if len(req.State) > MaxStateBytes {
		s.writeError(w, http.StatusRequestEntityTooLarge, "состояние слишком велико")
		return
	}

	if err := s.Store.SaveLeaderState(r.Context(), leader.ID, req.State); err != nil {
		s.Log.Error("не удалось сохранить состояние", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось сохранить состояние")
		return
	}
	s.writeJSON(w, http.StatusNoContent, nil)
}

// handleCreateReflection добавляет запись из тренажёра.
func (s *Server) handleCreateReflection(w http.ResponseWriter, r *http.Request) {
	leader, _ := LeaderFrom(r.Context())

	var req createReflectionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "не удалось разобрать запись")
		return
	}

	// Код — метка, по которой запись потом подбирается на экране тренажёра.
	// Сверять её со словарём компетенций нельзя: тренажёр помечает записи и
	// ключами сценариев (например, DESTRUCTOR_VIS), которых в словаре нет и
	// не должно быть. Поэтому проверяется форма, а не принадлежность:
	// короткая ASCII-метка без пробелов и разделителей.
	code := strings.TrimSpace(req.Code)
	if !validReflectionCode(code) {
		s.writeError(w, http.StatusBadRequest, "некорректный код записи")
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		s.writeError(w, http.StatusBadRequest, "пустая запись")
		return
	}
	if utf8.RuneCountInString(text) > MaxReflectionLength {
		s.writeError(w, http.StatusBadRequest, "запись слишком длинная")
		return
	}

	saved, err := s.Store.AddReflection(r.Context(), leader.ID, code, text)
	if err != nil {
		s.Log.Error("не удалось сохранить запись", "err", err)
		s.writeError(w, http.StatusInternalServerError, "не удалось сохранить запись")
		return
	}

	s.writeJSON(w, http.StatusCreated, reflectionView{
		Code:      saved.Code,
		Text:      saved.Text,
		CreatedAt: saved.CreatedAt,
	})
}
