import { useStore, hasCriticalDestructor } from '../state/store.jsx';
import { useProfile } from '../state/useProfile.js';
import { COMPANIONS, nameOf, rationale } from '../data';
import { Card, Banner, Button } from '../components/ui.jsx';

export default function PlanScreen() {
  const { state, dispatch } = useStore();
  const profile = useProfile();
  const blocked = hasCriticalDestructor(state, profile.destructors);

  if (blocked) {
    return (
      <section>
        <h1 className="scr-title">План развития</h1>
        <Banner title="Сначала — критическая зона">
          План развития суперсилы активируется после того, как деструктор проработан.
        </Banner>
      </section>
    );
  }

  if (!state.growthPoint) {
    return (
      <section>
        <h1 className="scr-title">План развития</h1>
        <Card>
          <div className="muted">
            Точка роста ещё не выбрана.{' '}
            <Button variant="secondary" onClick={() => dispatch({ type: 'SET_SCREEN', screen: 'growth' })}>
              Выбрать →
            </Button>
          </div>
        </Card>
      </section>
    );
  }

  const code = state.growthPoint;
  const base = profile.competencies.find((c) => c.code === code);
  const companions = (COMPANIONS[code] || [])
    .map((cc) => ({ code: cc, pct: profile.competencies.find((c) => c.code === cc)?.percentile ?? null }))
    .sort((a, b) => (b.pct ?? 0) - (a.pct ?? 0))
    .slice(0, 3);

  return (
    <section>
      <h1 className="scr-title">План развития</h1>
      <p className="scr-sub">
        Развитие уже сильной компетенции до «выдающегося» уровня (90+ перцентиль) идёт через
        компетенции-спутники, а не через универсальные тренинги.
      </p>

      <Card>
        <div className="sec-label">Выбранная точка роста</div>
        <h3 style={{ fontSize: 16 }}>
          {base.name} <span className="muted" style={{ fontWeight: 400 }}>— {base.percentile}-й перцентиль</span>
        </h3>
        <p className="muted" style={{ fontSize: 12.5 }}>
          Цель — довести восприятие этой компетенции с {base.percentile}-го до 90+ перцентиля через компетенции-спутники.
        </p>
      </Card>

      <Card>
        <h3>Рекомендованные компетенции-спутники</h3>
        {companions.map((comp, i) => (
          <div className="comp-card" key={comp.code}>
            <div className={`comp-rank ${i === 0 ? 'first' : ''}`}>{i + 1}</div>
            <div className="comp-body">
              <h4>{nameOf(comp.code)}</h4>
              <p>{rationale(code, comp.code)}</p>
              <div className="pct">Ваш текущий перцентиль: {comp.pct ?? '—'}</div>
            </div>
          </div>
        ))}
        <div className="btn-row">
          <Button
            onClick={() => {
              dispatch({ type: 'SET_TRAINER_SCENARIO', code });
              dispatch({ type: 'SET_SCREEN', screen: 'trainer' });
            }}
          >
            Перейти к тренажёру →
          </Button>
        </div>
      </Card>
    </section>
  );
}
