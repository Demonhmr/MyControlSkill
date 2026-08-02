import { useStore } from '../state/store.jsx';
import { clearState } from '../services/api.js';

const STEPS = [
  { id: 'survey', n: 1, label: 'Опрос 360°' },
  { id: 'destructors', n: 2, label: 'Деструкторы' },
  { id: 'strength', n: 3, label: 'Карта силы' },
  { id: 'growth', n: 4, label: 'Точка роста' },
  { id: 'plan', n: 5, label: 'План развития' },
  { id: 'trainer', n: 6, label: 'Тренажёр' },
  { id: 'pulse', n: 7, label: 'Пульс-трекер' },
  { id: 'hr', n: 8, label: 'HR-дашборд' },
];

export default function TopNav() {
  const { state, dispatch } = useStore();

  const toggleTheme = () => {
    const html = document.documentElement;
    const cur = html.getAttribute('data-theme');
    html.setAttribute('data-theme', cur === 'dark' ? 'light' : 'dark');
  };

  const reset = () => {
    if (!confirm('Сбросить весь демо-прогресс?')) return;
    clearState();
    dispatch({ type: 'RESET_DEMO' });
  };

  return (
    <header className="top">
      <div className="top-row">
        <div className="brand">
          Компас руководителя <span>— React-прототип</span>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="theme-btn" onClick={reset}>⟲ Сбросить демо</button>
          <button className="theme-btn" onClick={toggleTheme}>🌓 Тема</button>
        </div>
      </div>
      <nav className="steps">
        {STEPS.map((s) => (
          <button
            key={s.id}
            className={state.screen === s.id ? 'active' : ''}
            onClick={() => dispatch({ type: 'SET_SCREEN', screen: s.id })}
          >
            <span className="n">{s.n}</span> {s.label}
          </button>
        ))}
      </nav>
    </header>
  );
}
