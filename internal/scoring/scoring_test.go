package scoring

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"

	"mycontrolskill/internal/domain"
)

// Эталон снят с клиентской реализации скриптом scripts/gen-golden.sh.
// Расхождение здесь означает, что .exe и веб покажут одному руководителю
// разные перцентили; чинить надо реализацию, а не эталон.
const goldenPath = "testdata/golden.json"

type goldenScore struct {
	Code       string   `json:"code"`
	Raw        *float64 `json:"raw"`
	Percentile *int     `json:"percentile"`
}

type goldenAnswer struct {
	Kind      string `json:"kind"`
	Code      string `json:"code"`
	ItemIndex int    `json:"itemIndex"`
	Value     *int   `json:"value"`
}

type goldenResponse struct {
	Role    string         `json:"role"`
	Answers []goldenAnswer `json:"answers"`
}

type goldenFile struct {
	PercentileRank []struct {
		Code       string  `json:"code"`
		Value      float64 `json:"value"`
		Percentile int     `json:"percentile"`
	} `json:"percentileRank"`
	Profiles []struct {
		Name      string           `json:"name"`
		Responses []goldenResponse `json:"responses"`
		Profile   struct {
			Ready           bool          `json:"ready"`
			RespondentCount int           `json:"respondentCount"`
			Competencies    []goldenScore `json:"competencies"`
			Destructors     []goldenScore `json:"destructors"`
			BlindSpots      []BlindSpot   `json:"blindSpots"`
		} `json:"profile"`
	} `json:"profiles"`
}

func loadGolden(t *testing.T) goldenFile {
	t.Helper()
	body, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("не прочитан эталон: %v", err)
	}
	var g goldenFile
	if err := json.Unmarshal(body, &g); err != nil {
		t.Fatalf("разбор эталона: %v", err)
	}
	if len(g.PercentileRank) == 0 || len(g.Profiles) == 0 {
		t.Fatal("эталон пуст")
	}
	return g
}

func toDomain(responses []goldenResponse) []domain.ScoredResponse {
	out := make([]domain.ScoredResponse, 0, len(responses))
	for _, r := range responses {
		sr := domain.ScoredResponse{Role: domain.Role(r.Role)}
		for _, a := range r.Answers {
			sr.Answers = append(sr.Answers, domain.Answer{
				Kind:      domain.Kind(a.Kind),
				Code:      a.Code,
				ItemIndex: a.ItemIndex,
				Value:     a.Value,
			})
		}
		out = append(out, sr)
	}
	return out
}

// Самая рискованная часть порта: перцентиль считается относительно
// синтетической популяции из mulberry32 и преобразования Бокса — Мюллера.
// Разойтись реализации могут на любой из этих операций.
func TestПерцентильСовпадаетСКлиентским(t *testing.T) {
	g := loadGolden(t)

	mismatches := 0
	for _, v := range g.PercentileRank {
		got := percentileRank(v.Value, v.Code)
		if got != v.Percentile {
			mismatches++
			if mismatches <= 10 {
				t.Errorf("percentileRank(%g, %q) = %d, в JS %d", v.Value, v.Code, got, v.Percentile)
			}
		}
	}
	if mismatches > 10 {
		t.Errorf("…всего расхождений: %d из %d", mismatches, len(g.PercentileRank))
	}
}

func TestПрофильСовпадаетСКлиентским(t *testing.T) {
	g := loadGolden(t)

	for _, scenario := range g.Profiles {
		t.Run(scenario.Name, func(t *testing.T) {
			got := Compute(toDomain(scenario.Responses))
			want := scenario.Profile

			if got.Ready != want.Ready {
				t.Errorf("ready = %v, в JS %v", got.Ready, want.Ready)
			}
			if got.RespondentCount != want.RespondentCount {
				t.Errorf("respondentCount = %d, в JS %d", got.RespondentCount, want.RespondentCount)
			}

			compareScores(t, "компетенции", got.Competencies, want.Competencies)
			compareScores(t, "деструкторы", got.Destructors, want.Destructors)
			compareBlindSpots(t, got.BlindSpots, want.BlindSpots)
		})
	}
}

// Сверка по коду, а не по позиции: клиент выводит деструкторы в порядке
// демонстрационных перцентилей, сервер — в порядке словаря.
func compareScores(t *testing.T, what string, got []Score, want []goldenScore) {
	t.Helper()

	byCode := make(map[string]Score, len(got))
	for _, s := range got {
		byCode[s.Code] = s
	}
	if len(got) != len(want) {
		t.Errorf("%s: элементов %d, в JS %d", what, len(got), len(want))
	}

	for _, w := range want {
		g, ok := byCode[w.Code]
		if !ok {
			t.Errorf("%s: кода %q нет в расчёте", what, w.Code)
			continue
		}
		if !sameIntPtr(g.Percentile, w.Percentile) {
			t.Errorf("%s/%s: перцентиль %s, в JS %s", what, w.Code, showInt(g.Percentile), showInt(w.Percentile))
		}
		if !sameFloatPtr(g.Raw, w.Raw) {
			t.Errorf("%s/%s: среднее %s, в JS %s", what, w.Code, showFloat(g.Raw), showFloat(w.Raw))
		}
	}
}

func compareBlindSpots(t *testing.T, got, want []BlindSpot) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("слепых зон %d, в JS %d", len(got), len(want))
	}
	byCode := make(map[string]BlindSpot, len(got))
	for _, b := range got {
		byCode[b.Code] = b
	}
	for _, w := range want {
		g, ok := byCode[w.Code]
		if !ok {
			t.Errorf("слепая зона %q не найдена", w.Code)
			continue
		}
		if g != w {
			t.Errorf("слепая зона %q: %+v, в JS %+v", w.Code, g, w)
		}
	}
}

func sameIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// Среднее — результат деления, поэтому сравнение точное было бы придиркой
// к последнему биту; целый перцентиль от такой разницы не меняется.
func sameFloatPtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return math.Abs(*a-*b) < 1e-9
}

func showInt(v *int) string {
	if v == nil {
		return "—"
	}
	return strconv.Itoa(*v)
}

func showFloat(v *float64) string {
	if v == nil {
		return "—"
	}
	return strconv.FormatFloat(*v, 'g', -1, 64)
}
