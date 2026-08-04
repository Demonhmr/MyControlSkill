// Снимает эталонные векторы с клиентской реализации скоринга.
//
// Клиент и сервер считают профиль двумя разными реализациями: JS остаётся
// ради демо-режима без бэкенда, Go считает на сервере. Разъехаться они не
// имеют права — иначе один и тот же руководитель увидит разные перцентили в
// .exe и в вебе. Этот файл фиксирует, что именно выдаёт JS, а Go-тест
// (internal/scoring/scoring_test.go) сверяется с результатом.
//
// Запуск (нужны зависимости app/):
//
//     ./scripts/gen-golden.sh
//
// Перегенерировать эталон осмысленно только вместе с осознанным изменением
// методики: если Go-тест упал, сначала разбираемся, какая из реализаций
// неправа, и только потом трогаем этот файл.
import { writeFileSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { computeProfile } from '../app/src/logic/scoring.js';
import { percentileRank } from '../app/src/data/normPopulation.js';
import { COMPETENCIES, DESTRUCTORS } from '../app/src/data/index.js';

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const OUT = resolve(ROOT, 'internal/scoring/testdata/golden.json');

const COMPETENCY_CODES = COMPETENCIES.map((c) => c.code);
const DESTRUCTOR_CODES = DESTRUCTORS.map((d) => d.id);
const ALL_CODES = [...COMPETENCY_CODES, ...DESTRUCTOR_CODES];

// Сетка по всей шкале с мелким шагом: она проверяет сам генератор
// популяции, а не только итоговый расчёт. Дробные значения намеренно
// «некруглые» — средние по анкетам почти всегда такие.
function percentileVectors() {
  const out = [];
  for (const code of ALL_CODES) {
    for (let v = 1; v <= 5.0001; v += 0.05) {
      const value = Math.round(v * 1e6) / 1e6;
      out.push({ code, value, percentile: percentileRank(value, code) });
    }
    for (const value of [1.333333, 2.5, 3.1666666666666665, 3.75, 4.833333333333333]) {
      out.push({ code, value, percentile: percentileRank(value, code) });
    }
  }
  return out;
}

// Ответ в нейтральной форме: её же читает Go-тест, поэтому форма анкеты
// описывается один раз и не зависит от внутреннего вида ни одной из сторон.
function answer(kind, code, itemIndex, value) {
  return { kind, code, itemIndex, value };
}

function rate(kind, codes, values) {
  const out = [];
  for (const code of codes) {
    values.forEach((value, itemIndex) => out.push(answer(kind, code, itemIndex, value)));
  }
  return out;
}

function response(role, answers) {
  return { role, answers };
}

const scenarios = [
  {
    name: 'нет ответов вообще',
    responses: [],
  },
  {
    name: 'одна анкета — порог не пройден',
    responses: [response('peer', rate('competency', ['COM'], [5, 5]))],
  },
  {
    name: 'две внешних анкеты — порог всё ещё не пройден',
    responses: [
      response('peer', rate('competency', COMPETENCY_CODES, [5, 4])),
      response('subordinate', rate('competency', COMPETENCY_CODES, [4, 4])),
    ],
  },
  {
    name: 'три внешних анкеты — профиль считается',
    responses: [
      response('peer', [
        ...rate('competency', COMPETENCY_CODES, [5, 5]),
        ...rate('destructor', DESTRUCTOR_CODES, [2, 1]),
      ]),
      response('subordinate', [
        ...rate('competency', COMPETENCY_CODES, [5, 4]),
        ...rate('destructor', DESTRUCTOR_CODES, [3, 2]),
      ]),
      response('manager', [
        ...rate('competency', COMPETENCY_CODES, [4, 5]),
        ...rate('destructor', DESTRUCTOR_CODES, [1, 1]),
      ]),
    ],
  },
  {
    name: 'самооценка не входит в расчёт и даёт слепые зоны',
    responses: [
      response('peer', rate('competency', COMPETENCY_CODES, [5, 5])),
      response('subordinate', rate('competency', COMPETENCY_CODES, [5, 5])),
      response('manager', rate('competency', COMPETENCY_CODES, [5, 5])),
      response('self', rate('competency', COMPETENCY_CODES, [1, 1])),
    ],
  },
  {
    name: 'самооценка выше внешней',
    responses: [
      response('peer', rate('competency', COMPETENCY_CODES, [2, 1])),
      response('subordinate', rate('competency', COMPETENCY_CODES, [1, 2])),
      response('manager', rate('competency', COMPETENCY_CODES, [2, 2])),
      response('self', rate('competency', COMPETENCY_CODES, [5, 5])),
    ],
  },
  {
    name: '«не могу оценить» не идёт в среднее',
    responses: [
      response('peer', [
        answer('competency', 'COM', 0, 5),
        answer('competency', 'COM', 1, null),
        answer('competency', 'INT', 0, null),
        answer('competency', 'INT', 1, null),
      ]),
      response('subordinate', [
        answer('competency', 'COM', 0, 4),
        answer('competency', 'COM', 1, 4),
      ]),
      response('manager', [
        answer('competency', 'COM', 0, 3),
        answer('competency', 'COM', 1, null),
      ]),
    ],
  },
  {
    name: 'неполные анкеты и дробные средние',
    responses: [
      response('peer', [
        answer('competency', 'VIS', 0, 1),
        answer('competency', 'VIS', 1, 2),
        answer('destructor', 'd3', 0, 1),
      ]),
      response('subordinate', [
        answer('competency', 'VIS', 0, 2),
        answer('destructor', 'd3', 0, 2),
        answer('destructor', 'd3', 1, 1),
      ]),
      response('manager', [
        answer('competency', 'VIS', 0, 4),
        answer('competency', 'VIS', 1, 5),
        answer('destructor', 'd3', 1, 5),
      ]),
      response('self', [
        answer('competency', 'VIS', 0, 5),
        answer('competency', 'VIS', 1, 5),
      ]),
    ],
  },
  {
    name: 'только самооценка',
    responses: [response('self', rate('competency', COMPETENCY_CODES, [4, 4]))],
  },
];

// Перевод нейтральной формы в ту, которую ждёт computeProfile.
function toClientShape(responses) {
  return responses.map((r) => {
    const competencyScores = {};
    const destructorScores = {};
    for (const a of r.answers) {
      const target = a.kind === 'competency' ? competencyScores : destructorScores;
      if (!target[a.code]) target[a.code] = [null, null];
      target[a.code][a.itemIndex] = a.value;
    }
    return { role: r.role, tenure: '3to12', competencyScores, destructorScores, open1: '', open2: '' };
  });
}

// Перцентиль выгружается только когда он посчитан по живым данным.
//
// Ниже порога респондентов клиент подставляет демонстрационные значения из
// data/index.js — это витрина прототипа, а не результат замера, и сервер
// такие числа не отдаёт. Сверять их было бы сверкой демо-заглушки.
function toGolden(profile) {
  const scores = (items, key) =>
    items.map((item) => ({
      code: item[key],
      raw: item.raw ?? null,
      percentile: item.isLive ? item.percentile : null,
    }));

  return {
    ready: profile.ready,
    respondentCount: profile.respondentCount,
    competencies: scores(profile.competencies, 'code'),
    destructors: scores(profile.destructors, 'id'),
    blindSpots: profile.blindSpots.map((b) => ({
      code: b.code,
      selfPercentile: b.selfPct,
      othersPercentile: b.othersPct,
      delta: b.delta,
    })),
  };
}

const golden = {
  note: 'Сгенерировано scripts/gen-golden.sh из клиентской реализации. Руками не править.',
  percentileRank: percentileVectors(),
  profiles: scenarios.map((s) => ({
    name: s.name,
    responses: s.responses,
    profile: toGolden(computeProfile(toClientShape(s.responses))),
  })),
};

mkdirSync(dirname(OUT), { recursive: true });
writeFileSync(OUT, JSON.stringify(golden, null, 2) + '\n');

console.log(`Записано: ${OUT}`);
console.log(`  векторов percentileRank: ${golden.percentileRank.length}`);
console.log(`  сценариев профиля:       ${golden.profiles.length}`);
