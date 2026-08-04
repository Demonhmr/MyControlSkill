// Обращения к серверу от лица респондента.
//
// Отдельно от services/api.js: тот обслуживает рабочее состояние
// руководителя и умеет работать без сети (демо в .exe), а анкета по ссылке
// без сервера не существует в принципе.

const COMPETENCY = 'competency';
const DESTRUCTOR = 'destructor';

class SurveyError extends Error {
  constructor(message, status) {
    super(message);
    this.name = 'SurveyError';
    this.status = status;
  }
}

async function parse(response) {
  let body = null;
  try {
    body = await response.json();
  } catch {
    // Тело может быть пустым или не-JSON — сообщение возьмём из статуса.
  }
  if (!response.ok) {
    throw new SurveyError(body?.error ?? 'Сервер вернул ошибку', response.status);
  }
  return body;
}

/** Контекст анкеты: роль, кого оценивают, не погашена ли ссылка. */
export async function fetchSurvey(token) {
  const response = await fetch(`/api/survey/${encodeURIComponent(token)}`, {
    headers: { Accept: 'application/json' },
  });
  return parse(response);
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
export async function submitSurvey(token, form) {
  const response = await fetch(`/api/survey/${encodeURIComponent(token)}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    body: JSON.stringify(toSubmission(form)),
  });
  return parse(response);
}
