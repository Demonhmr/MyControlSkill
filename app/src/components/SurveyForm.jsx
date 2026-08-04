import { COMPETENCIES, DESTRUCTORS } from '../data';
import { COMPETENCY_ITEMS, DESTRUCTOR_ITEMS, OPEN_QUESTIONS, ROLES, TENURE } from '../data/questionnaire';
import { Card, Button } from './ui.jsx';
import RatingScale from './RatingScale.jsx';

const CLUSTERS = [...new Set(COMPETENCIES.map((c) => c.cluster))];

export function emptyForm() {
  return {
    role: '',
    tenure: '',
    competencyScores: {},
    destructorScores: {},
    open1: '',
    open2: '',
  };
}

/**
 * Форма анкеты 360°.
 *
 * Один и тот же набор вопросов заполняют двое: руководитель в демо-режиме и
 * приглашённый респондент по своей ссылке. Отличаются они только тем, откуда
 * берётся роль — в демо её выбирают здесь, респонденту её назначил
 * руководитель при приглашении, и подменить её нельзя.
 */
export default function SurveyForm({
  form,
  setForm,
  onSubmit,
  showRole = true,
  disabled = false,
  submitLabel = 'Отправить анкету',
}) {
  const setScore = (mapKey, key, idx, val) => {
    setForm((f) => {
      const arr = f[mapKey][key] ? [...f[mapKey][key]] : [undefined, undefined];
      arr[idx] = val;
      return { ...f, [mapKey]: { ...f[mapKey], [key]: arr } };
    });
  };

  const canSubmit = (!showRole || form.role) && form.tenure && !disabled;

  return (
    <>
      <Card>
        {showRole && (
          <>
            <div className="sec-label">Кто заполняет</div>
            <div className="chips">
              {ROLES.map((r) => (
                <button
                  key={r.value}
                  className={`chip ${form.role === r.value ? 'selected' : ''}`}
                  onClick={() => setForm((f) => ({ ...f, role: r.value }))}
                >
                  {r.label}
                </button>
              ))}
            </div>
          </>
        )}
        <div className="sec-label">Как долго наблюдаете этого руководителя</div>
        <div className="chips">
          {TENURE.map((t) => (
            <button
              key={t.value}
              className={`chip ${form.tenure === t.value ? 'selected' : ''}`}
              onClick={() => setForm((f) => ({ ...f, tenure: t.value }))}
            >
              {t.label}
            </button>
          ))}
        </div>
      </Card>

      {CLUSTERS.map((cluster) => (
        <Card key={cluster}>
          <h3>{cluster}</h3>
          {COMPETENCIES.filter((c) => c.cluster === cluster).map((c) => (
            <div key={c.code} style={{ marginBottom: 14 }}>
              {COMPETENCY_ITEMS[c.code].map((item, idx) => (
                <div key={idx} style={{ marginBottom: 8 }}>
                  <div style={{ fontSize: 13 }}>{item}</div>
                  <RatingScale
                    value={form.competencyScores[c.code]?.[idx]}
                    onChange={(v) => setScore('competencyScores', c.code, idx, v)}
                  />
                </div>
              ))}
            </div>
          ))}
        </Card>
      ))}

      <Card>
        <h3>Критические зоны (деструкторы)</h3>
        <p className="muted" style={{ fontSize: 12, marginTop: -6 }}>
          Как часто вы наблюдаете это поведение
        </p>
        {DESTRUCTORS.map((d) => (
          <div key={d.id} style={{ marginBottom: 14 }}>
            <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4 }}>{d.name}</div>
            {DESTRUCTOR_ITEMS[d.id].map((item, idx) => (
              <div key={idx} style={{ marginBottom: 8 }}>
                <div style={{ fontSize: 13 }}>{item}</div>
                <RatingScale
                  value={form.destructorScores[d.id]?.[idx]}
                  onChange={(v) => setScore('destructorScores', d.id, idx, v)}
                />
              </div>
            ))}
          </div>
        ))}
      </Card>

      <Card>
        <h3>Открытые вопросы</h3>
        {OPEN_QUESTIONS.map((q, i) => (
          <div key={i} style={{ marginBottom: 12 }}>
            <div style={{ fontSize: 13, marginBottom: 6 }}>{q}</div>
            <textarea
              className="reflect"
              value={i === 0 ? form.open1 : form.open2}
              onChange={(e) => setForm((f) => ({ ...f, [i === 0 ? 'open1' : 'open2']: e.target.value }))}
            />
          </div>
        ))}
        <div className="btn-row">
          <Button disabled={!canSubmit} onClick={onSubmit}>
            {submitLabel}
          </Button>
        </div>
        {!canSubmit && !disabled && (
          <div className="footnote">
            {showRole
              ? 'Укажите роль и срок наблюдения выше, чтобы отправить.'
              : 'Укажите срок наблюдения выше, чтобы отправить.'}
          </div>
        )}
      </Card>
    </>
  );
}
