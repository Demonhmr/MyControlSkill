import { useState } from 'react';
import { useStore } from '../state/store.jsx';
import { nameOf, scenarioFor } from '../data';
import { Card, Button } from '../components/ui.jsx';

export default function TrainerScreen() {
  const { state, dispatch } = useStore();
  const [draft, setDraft] = useState('');
  const code = state.trainerScenarioCode || state.growthPoint;

  if (!code) {
    return (
      <section>
        <h1 className="scr-title">Тренажёр «Трудный диалог»</h1>
        <Card>
          <div className="muted">
            Сначала выберите точку роста или откройте тренажёр из критической зоны на экране «Деструкторы».
          </div>
        </Card>
      </section>
    );
  }

  const sc = scenarioFor(code);
  const title = code === 'DESTRUCTOR_VIS' ? 'Критическая зона: нет ясного видения' : nameOf(code);
  const entries = state.reflections.filter((r) => r.code === code);

  const save = () => {
    const val = draft.trim();
    if (!val) return;
    dispatch({ type: 'ADD_REFLECTION', date: new Date().toLocaleDateString('ru-RU'), code, text: val });
    setDraft('');
  };

  return (
    <section>
      <h1 className="scr-title">Тренажёр «Трудный диалог»</h1>
      <p className="scr-sub">
        Наибольшее впечатление формируют ситуации несогласия и эмоционального напряжения — это главный
        тренажёр для перехода в разряд «выдающихся».
      </p>

      <Card>
        <div className="sec-label">{code === 'DESTRUCTOR_VIS' ? 'Тренажёр по деструктору' : 'Тренажёр по точке роста'}</div>
        <h3 style={{ fontSize: 16 }}>{title}</h3>
        <div className="dlg-step"><div className="k">Ситуация</div><div className="v">{sc.trigger}</div></div>
        <div className="dlg-step"><div className="k">Не делать</div><div className="v bad">{sc.bad}</div></div>
        <div className="dlg-step"><div className="k">Выдающийся уровень</div><div className="v good">{sc.good}</div></div>
        <div className="dlg-step"><div className="k">Скрипт</div><div className="v script">{sc.script}</div></div>
      </Card>

      <Card>
        <h3>После разговора — рефлексия</h3>
        <textarea
          className="reflect"
          placeholder="Что сказал(а) собеседник? Что вы почувствовали? Что сделаете иначе в следующий раз?"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
        />
        <div className="btn-row">
          <Button onClick={save}>Сохранить запись</Button>
        </div>
        {entries.map((r, i) => (
          <div className="log-entry" key={i}>
            <div className="meta">{r.date}</div>
            <div>{r.text}</div>
          </div>
        ))}
      </Card>
    </section>
  );
}
