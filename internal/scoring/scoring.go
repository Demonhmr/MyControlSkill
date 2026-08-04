package scoring

import (
	"mycontrolskill/internal/domain"
)

// BlindSpotThreshold — расхождение самооценки и внешних оценок в
// перцентилях, начиная с которого это считается слепой зоной.
const BlindSpotThreshold = 20

// Score — оценка одной компетенции или одного деструктора.
type Score struct {
	Code string `json:"code"`
	// Raw — среднее по пунктам анкеты, nil если ни одной оценки нет.
	Raw *float64 `json:"raw"`
	// Percentile — место в нормативной популяции. nil, пока внешних анкет
	// меньше порога: до него любое число было бы отчётом по шуму.
	Percentile *int `json:"percentile"`
}

// IsLive сообщает, посчитан ли перцентиль по реальным данным.
func (s Score) IsLive() bool { return s.Percentile != nil }

// BlindSpot — компетенция, которую руководитель и окружение видят по-разному.
type BlindSpot struct {
	Code string `json:"code"`
	// SelfPercentile — перцентиль по самооценке.
	SelfPercentile int `json:"selfPercentile"`
	// OthersPercentile — перцентиль по внешним оценкам.
	OthersPercentile int `json:"othersPercentile"`
	// Delta положительна, когда руководитель оценивает себя выше окружения.
	Delta int `json:"delta"`
}

// Profile — результат расчёта по раунду 360°.
//
// Сырых ответов здесь нет и быть не должно: это ровно тот объём, который
// уходит клиенту.
type Profile struct {
	Competencies []Score `json:"competencies"`
	Destructors  []Score `json:"destructors"`
	// RespondentCount — число внешних анкет; самооценка в него не входит.
	RespondentCount int `json:"respondentCount"`
	// Ready — набралось ли внешних анкет на расчёт.
	Ready      bool        `json:"ready"`
	BlindSpots []BlindSpot `json:"blindSpots"`
}

// Compute считает профиль по анкетам раунда.
//
// Пока внешних анкет меньше domain.MinRespondents, перцентили не считаются
// вовсе. Клиентская версия в этом случае подставляет демонстрационные
// значения из data/index.js — на сервере такой подстановки нет намеренно:
// выдавать выдуманные числа за результат замера нельзя.
func Compute(responses []domain.ScoredResponse) Profile {
	var external []domain.ScoredResponse
	var self *domain.ScoredResponse
	for i, r := range responses {
		if r.Role == domain.RoleSelf {
			if self == nil {
				self = &responses[i]
			}
			continue
		}
		if r.Role.IsExternal() {
			external = append(external, r)
		}
	}

	ready := len(external) >= domain.MinRespondents

	profile := Profile{
		Competencies:    make([]Score, 0, len(domain.CompetencyCodes)),
		Destructors:     make([]Score, 0, len(domain.DestructorCodes)),
		RespondentCount: len(external),
		Ready:           ready,
		BlindSpots:      []BlindSpot{},
	}

	for _, code := range domain.CompetencyCodes {
		profile.Competencies = append(profile.Competencies, score(external, domain.KindCompetency, code, ready))
	}
	for _, code := range domain.DestructorCodes {
		profile.Destructors = append(profile.Destructors, score(external, domain.KindDestructor, code, ready))
	}

	if self != nil {
		profile.BlindSpots = blindSpots(*self, profile.Competencies)
	}
	return profile
}

// score считает средний балл и перцентиль по одному коду.
func score(external []domain.ScoredResponse, kind domain.Kind, code string, ready bool) Score {
	s := Score{Code: code}
	if !ready {
		return s
	}

	avg, ok := average(external, kind, code)
	if !ok {
		// Код есть в анкете, но его никто не оценил.
		return s
	}
	s.Raw = &avg

	p := percentileRank(avg, code)
	s.Percentile = &p
	return s
}

// average усредняет оценки по всем пунктам кода и по всем анкетам сразу.
//
// Именно так, а не средним из средних по респондентам: респондент,
// оценивший один пункт из двух, влияет на результат вдвое слабее того, кто
// оценил оба. Так же считает и клиентская версия.
func average(responses []domain.ScoredResponse, kind domain.Kind, code string) (float64, bool) {
	var sum float64
	var n int
	for _, r := range responses {
		for _, a := range r.Answers {
			if a.Kind != kind || a.Code != code || a.Value == nil {
				continue
			}
			sum += float64(*a.Value)
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

// blindSpots ищет компетенции, где самооценка сильно расходится с внешней.
func blindSpots(self domain.ScoredResponse, competencies []Score) []BlindSpot {
	spots := []BlindSpot{}
	selfOnly := []domain.ScoredResponse{self}

	for _, c := range competencies {
		if !c.IsLive() {
			continue
		}
		selfAvg, ok := average(selfOnly, domain.KindCompetency, c.Code)
		if !ok {
			continue
		}

		selfPct := percentileRank(selfAvg, c.Code)
		delta := selfPct - *c.Percentile
		if delta <= -BlindSpotThreshold || delta >= BlindSpotThreshold {
			spots = append(spots, BlindSpot{
				Code:             c.Code,
				SelfPercentile:   selfPct,
				OthersPercentile: *c.Percentile,
				Delta:            delta,
			})
		}
	}
	return spots
}
