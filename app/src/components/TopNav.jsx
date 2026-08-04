import { useStore } from '../state/store.jsx';
import { useSession } from '../state/session.jsx';
import { clearState } from '../services/api.js';
import { logout } from '../services/leaderApi.js';

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

// Раунды существуют только там, где есть бэкенд: в .exe приглашать некого.
const SERVER_STEP = { id: 'rounds', n: '★', label: 'Раунды 360°' };

export default function TopNav() {
  const { state, dispatch } = useStore();
  const { mode, leader, refresh } = useSession();

  // «Опрос 360°» в сетевом режиме не нужен: анкеты заполняют приглашённые по
  // своим ссылкам, а этот экран пишет в localStorage и ни на что не влияет.
  const steps =
    mode === 'server' ? [SERVER_STEP, ...STEPS.filter((s) => s.id !== 'survey')] : STEPS;

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

  const signOut = async () => {
    try {
      await logout();
    } finally {
      await refresh();
    }
  };

  return (
    <header className="top">
      <div className="top-row">
        <div className="brand">
          Компас руководителя{' '}
          <span>{mode === 'server' ? `— ${leader?.email ?? ''}` : '— React-прототип'}</span>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          {mode === 'local' && (
            <button className="theme-btn" onClick={reset}>⟲ Сбросить демо</button>
          )}
          {mode === 'server' && (
            <button className="theme-btn" onClick={signOut}>Выйти</button>
          )}
          <button className="theme-btn" onClick={toggleTheme}>🌓 Тема</button>
        </div>
      </div>
      <nav className="steps">
        {steps.map((s) => (
          <button
            key={s.id}
            className={
              state.screen === s.id || (mode === 'server' && state.screen === 'survey' && s.id === 'rounds')
                ? 'active'
                : ''
            }
            onClick={() => dispatch({ type: 'SET_SCREEN', screen: s.id })}
          >
            <span className="n">{s.n}</span> {s.label}
          </button>
        ))}
      </nav>
    </header>
  );
}
