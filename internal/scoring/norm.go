// Пакет scoring считает профиль руководителя по собранным анкетам 360°.
//
// Это порт app/src/logic/scoring.js и app/src/data/normPopulation.js.
// Расчёт переехал на сервер не ради скорости: чтобы посчитать профиль на
// клиенте, пришлось бы отдать ему сырые ответы, а по ним руководитель
// восстановил бы, кто именно как отвечал.
//
// Клиентская реализация остаётся — она обслуживает демо-режим без бэкенда.
// Две реализации обязаны совпадать до целого перцентиля; за этим следит
// scoring_test.go по эталонным векторам из testdata/golden.json.
package scoring

import (
	"math"
	"sort"
	"unicode/utf16"

	"mycontrolskill/internal/domain"
)

// Параметры синтетической нормативной популяции.
//
// Она временная: перцентили пока считаются относительно сгенерированного
// распределения, а не реальных данных. Калибровка на собранных анкетах —
// отдельная задача (Фаза 2), и менять эти константы до неё нельзя, иначе
// перцентили поедут у всех уже посчитанных профилей.
const (
	populationSize = 300
	populationMean = 3.2
	populationSD   = 0.55
)

// mulberry32 — тот же генератор, что в normPopulation.js.
//
// Все операции в 32 битах без знака: в JS там `|0`, `>>>` и Math.imul,
// которые дают ровно те же наборы бит. Сложение в JS идёт через float64,
// но результат тут же усекается до 32 бит операцией `^`, поэтому
// беззнаковое сложение с переполнением ему эквивалентно.
type mulberry32 struct {
	state uint32
}

func (m *mulberry32) next() float64 {
	m.state += 0x6D2B79F5
	t := (m.state ^ (m.state >> 15)) * (1 | m.state)
	t = (t + (t^(t>>7))*(61|t)) ^ t
	return float64(t^(t>>14)) / 4294967296
}

// codeSeed повторяет хэш строки из normPopulation.js.
//
// В JS charCodeAt отдаёт кодовую единицу UTF-16, поэтому здесь строка
// раскладывается так же. Коды компетенций латинские, но завязываться на
// это молча не стоит.
func codeSeed(code string) uint32 {
	var h uint32
	for _, unit := range utf16.Encode([]rune(code)) {
		h = h*31 + uint32(unit)
	}
	return h
}

// buildPopulation строит отсортированное распределение оценок для кода.
// Значения нормальные (преобразование Бокса — Мюллера), обрезанные шкалой.
func buildPopulation(code string) []float64 {
	rand := &mulberry32{state: codeSeed(code)}
	pop := make([]float64, 0, populationSize)
	for i := 0; i < populationSize; i++ {
		u1 := rand.next()
		if u1 == 0 {
			// В JS это `rand() || 1e-6`: логарифм нуля дал бы -Inf.
			u1 = 1e-6
		}
		u2 := rand.next()
		z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
		pop = append(pop, math.Max(domain.MinScore, math.Min(domain.MaxScore, populationMean+z*populationSD)))
	}
	sort.Float64s(pop)
	return pop
}

// populations построены заранее для всех известных кодов: их три десятка,
// это доли миллисекунды, зато не нужна блокировка на горячем пути.
var populations = buildAllPopulations()

func buildAllPopulations() map[string][]float64 {
	all := make(map[string][]float64, len(domain.CompetencyCodes)+len(domain.DestructorCodes))
	for _, code := range domain.CompetencyCodes {
		all[code] = buildPopulation(code)
	}
	for _, code := range domain.DestructorCodes {
		all[code] = buildPopulation(code)
	}
	return all
}

// percentileRank — доля популяции, оказавшаяся не выше value, в процентах.
func percentileRank(value float64, code string) int {
	pop, ok := populations[code]
	if !ok {
		// Код не из словаря: популяции нет, считать не от чего.
		pop = buildPopulation(code)
	}
	// Верхняя граница: сколько элементов <= value.
	below := sort.Search(len(pop), func(i int) bool { return pop[i] > value })
	return int(math.Round(float64(below) / float64(len(pop)) * 100))
}
