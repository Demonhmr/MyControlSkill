import { useState } from 'react';
import { useStore } from '../state/store.jsx';
import { COMPETENCIES, DESTRUCTORS } from '../data';
import { COMPETENCY_ITEMS, DESTRUCTOR_ITEMS, OPEN_QUESTIONS, ROLES, TENURE } from '../data/questionnaire';
import { MIN_RESPONDENTS } from '../logic/scoring';
import { Card, Button, Banner } from '../components/ui.jsx';
import RatingScale from '../components/RatingScale.jsx';

const CLUSTERS = [...new Set(COMPETENCIES.map((c) => c.cluster))];

function emptyForm() {
  return {
    role: '', tenure: '',
    competencyScores: {},
    destructorScores: {},
    open1: '', open2: '',
  };
}

export default function Survey360Screen() {
  const { state, dispatch } = useStore();
  const [form, setForm] = useState(emptyForm());
  const [submitted, setSubmitted] = useState(false);

  const setScore = (mapKey, key, idx, val) => {
    setForm((f) => {
      const arr = f[mapKey][key] ? [...f[mapKey][key]] : [null, null];
      arr[idx] = val;
      return { ...f, [mapKey]: { ...f[mapKey], [key]: arr } };
    });
  };

  const canSubmit = form.role && form.tenure;

  const submit = () => {
    if (!canSubmit) return;
    dispatch({ type: 'ADD_RESPONSE', response: { ...form, id: Date.now() } });
    setForm(emptyForm());
    setSubmitted(true);
    setTimeout(() => setSubmitted(false), 3000);
  };

  const externalCount = state.responses.filter((r) => r.role !== 'self').length;

  return (
    <section>
      <h1 className="scr-title">Опрос 360°</h1>
      <p className="scr-sub">
        Каждая карточка ниже — один заполненный опросник (один респондент). Собрано внешних ответов:{' '}
        <b>{externalCount}</b> из {MIN_RESPONDENTS} минимально необходимых для расчёта перцентиля.
      </p>

      {externalCount < MIN_RESPONDENTS && (
        <Banner title="Недостаточно данных">
          Пока внешних респондентов меньше {MIN_RESPONDENTS}, остальные экраны показывают демонстрационные
          значения, а не реальный расчёт — это намеренная защита от отчёта на шуме.
        </Banner>
      )}
      {submitted && <Banner tone="ok" title="Ответ сохранён">Спасибо — анкета учтена в профиле.</Banner>}

      <Card>
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
                    value={form.competencyScores[c.code]?.[idx] ?? undefined}
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
                  value={form.destructorScores[d.id]?.[idx] ?? undefined}
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
          <Button disabled={!canSubmit} onClick={submit}>Отправить анкету</Button>
        </div>
        {!canSubmit && <div className="footnote">Укажите роль и срок наблюдения выше, чтобы отправить.</div>}
      </Card>
    </section>
  );
}
