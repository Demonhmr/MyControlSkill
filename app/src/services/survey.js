// Обращения к серверу от лица респондента.
//
// Отдельно от services/api.js: тот обслуживает рабочее состояние
// руководителя и умеет работать без сети (демо в .exe), а анкета по ссылке
// без сервера не существует в принципе.

import { request } from './http.js';

const COMPETENCY = 'competency';
const DESTRUCTOR = 'destructor';

/** Контекст анкеты: роль, кого оценивают, не погашена ли ссылка. */
export function fetchSurvey(token) {
  return request(`/api/survey/${encodeURIComponent(token)}`);
}

/**
 * Превращает состояние формы в то, что ждёт сервер.
 *
 * Здесь важна разница между тремя состояниями пункта: не тронут (undefined —
 * не отправляем вовсе), «не могу оценить» (null — отправляем явно) и оценка.
 * Первые два одинаково не идут в среднее, но их доля показывает, насколько
 * респондент вообще наблюдает руководителя.
 */
export function toSubmission(form) {
  const answers = [];
  const collect = (kind, scores) => {
    for (const [code, values] of Object.entries(scores ?? {})) {
      (values ?? []).forEach((value, itemIndex) => {
        if (value === undefined) return;
        answers.push({ kind, code, itemIndex, value: value === null ? null : value });
      });
    }
  };
  collect(COMPETENCY, form.competencyScores);
  collect(DESTRUCTOR, form.destructorScores);

  const openAnswers = [
    { questionIndex: 0, text: form.open1 ?? '' },
    { questionIndex: 1, text: form.open2 ?? '' },
  ].filter((o) => o.text.trim() !== '');

  return { tenure: form.tenure, answers, openAnswers };
}

/** Отправляет заполненную анкету. Роль сервер берёт из приглашения. */
export function submitSurvey(token, form) {
  return request(`/api/survey/${encodeURIComponent(token)}`, {
    method: 'POST',
    body: toSubmission(form),
  });
}
