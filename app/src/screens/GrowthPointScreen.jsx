import { useStore, hasCriticalDestructor } from '../state/store.jsx';
import { useProfile } from '../state/useProfile.js';
import { NEEDS } from '../data';
import { Banner, Button } from '../components/ui.jsx';

export default function GrowthPointScreen() {
  const { state, dispatch } = useStore();
  const profile = useProfile();
  const blocked = hasCriticalDestructor(state, profile.destructors);

  if (blocked) {
    return (
      <section>
        <h1 className="scr-title">Выбор точки роста</h1>
        <Banner title="Раздел заблокирован">
          Сначала проработайте критическую зону на экране «Деструкторы» — это единственный приоритет по модели статьи.
          <div className="btn-row">
            <Button variant="secondary" onClick={() => dispatch({ type: 'SET_SCREEN', screen: 'destructors' })}>
              Перейти к деструкторам
            </Button>
          </div>
        </Banner>
      </section>
    );
  }

  const candidates = profile.competencies.filter((c) => c.percentile >= 70).sort((a, b) => b.percentile - a.percentile);
  const selectedNeedCodes = new Set(NEEDS.filter((n) => state.needs[n.id]).flatMap((n) => n.codes));
  const needsSelected = selectedNeedCodes.size > 0;

  return (
    <section>
      <h1 className="scr-title">Выбор точки роста</h1>
      <p className="scr-sub">
        Точка роста — компетенция на пересечении трёх фильтров. Отметьте, что вам интересно развивать,
        и выберите вызовы, которые сейчас стоят перед командой.
      </p>

      <div className="venn-hint">
        <div className="vi"><b>1. Сила</b>перцентиль ≥ 70 по данным 360° — кандидаты ниже</div>
        <div className="vi"><b>2. Интерес</b>отметьте чекбоксом на карточке компетенции</div>
        <div className="vi"><b>3. Потребность</b>выберите актуальные вызовы команды ниже</div>
      </div>

      <div className="sec-label">Вызовы команды сейчас</div>
      <div className="chips">
        {NEEDS.map((n) => (
          <button
            key={n.id}
            className={`chip ${state.needs[n.id] ? 'selected' : ''}`}
            onClick={() => dispatch({ type: 'TOGGLE_NEED', id: n.id })}
          >
            {n.label}
          </button>
        ))}
      </div>

      <div className="sec-label">Кандидаты (сила ≥ 70 перцентиль)</div>
      {candidates.length === 0 && <div className="muted">Пока нет компетенций выше 70 перцентиля — соберите больше данных 360°.</div>}
      <div className="gp-grid">
        {candidates.map((c) => {
          const interested = !!state.interest[c.code];
          const matchesNeed = selectedNeedCodes.has(c.code);
          const isMatch = interested && (!needsSelected || matchesNeed);

          return (
            <div className={`gp-card ${isMatch ? 'match' : ''}`} key={c.code}>
              <h4>{c.name}{c.percentile >= 90 ? ' ★' : ''}</h4>
              <div className="pct">{c.percentile}-й перцентиль · {c.cluster}</div>
              <label className="int">
                <input
                  type="checkbox"
                  checked={interested}
                  onChange={() => dispatch({ type: 'TOGGLE_INTEREST', code: c.code })}
                />
                Хочу развивать это
              </label>
              <div className={`needmatch ${needsSelected ? (matchesNeed ? 'yes' : 'no') : ''}`}>
                {needsSelected
                  ? (matchesNeed ? '✓ Совпадает с потребностью команды' : 'Не связано с выбранными вызовами')
                  : 'Выберите вызов команды выше, чтобы проверить'}
              </div>
              <Button
                variant={isMatch ? '' : 'ghost'}
                disabled={!isMatch}
                onClick={() => {
                  dispatch({ type: 'SET_GROWTH_POINT', code: c.code });
                  dispatch({ type: 'SET_SCREEN', screen: 'plan' });
                }}
              >
                Выбрать точкой роста
              </Button>
            </div>
          );
        })}
      </div>
    </section>
  );
}
