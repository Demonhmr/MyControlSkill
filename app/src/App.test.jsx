import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import App from './App.jsx';
import { htmlResponse, jsonResponse, routeFetch } from './test/helpers.jsx';

const LEADER = { id: 'l1', email: 'lead@example.com', name: '' };

const ROUND = (counts) => ({
  id: 'a1',
  title: 'Пилот',
  createdAt: '2026-08-01T10:00:00.000Z',
  closedAt: null,
  counts: { self: 0, required: 3, ...counts },
});

function liveProfile() {
  return {
    ready: true,
    respondentCount: 3,
    competencies: [{ code: 'COM', raw: 4.5, percentile: 88 }],
    destructors: [{ code: 'd3', raw: 1.2, percentile: 7 }],
    blindSpots: [],
  };
}

function renderApp(routes) {
  vi.stubGlobal('fetch', routeFetch(routes));
  render(<App />);
}

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
});

describe('App', () => {
  it('в локальном режиме открывает демо без входа', async () => {
    renderApp({ 'GET /api/me': htmlResponse() });

    expect(await screen.findByText('Опрос 360°', { selector: 'h1' })).toBeInTheDocument();
    expect(screen.queryByText('Прислать ссылку')).not.toBeInTheDocument();
  });

  it('в сетевом режиме без сессии требует вход', async () => {
    renderApp({ 'GET /api/me': jsonResponse({ error: 'требуется вход' }, 401) });

    expect(await screen.findByRole('button', { name: 'Прислать ссылку' })).toBeInTheDocument();
  });

  // Главное свойство сетевого режима: пока анкет мало, чисел не показываем
  // вовсе. Демонстрационные значения из data/index.js легко принять за
  // результат замера собственной команды.
  it('в сетевом режиме прячет экраны с числами до готовности профиля', async () => {
    renderApp({
      'GET /api/me': jsonResponse(LEADER),
      'GET /api/assessments': jsonResponse({ assessments: [ROUND({ external: 2, ready: false })] }),
      'GET /api/assessments/a1/profile': jsonResponse(
        { error: 'мало анкет', counts: { external: 2, self: 0, required: 3, ready: false } },
        423
      ),
    });

    // Заход начинается с раундов.
    expect(await screen.findByText('Раунды 360°', { selector: 'h1' })).toBeInTheDocument();

    const user = (await import('@testing-library/user-event')).default.setup();
    await user.click(screen.getByRole('button', { name: /Деструкторы/ }));

    expect(await screen.findByText('Собрано 2 из 3 анкет')).toBeInTheDocument();
    expect(screen.queryByText('Показаны демонстрационные значения')).not.toBeInTheDocument();
  });

  it('в сетевом режиме показывает числа с сервера, когда профиль готов', async () => {
    renderApp({
      'GET /api/me': jsonResponse(LEADER),
      'GET /api/assessments': jsonResponse({ assessments: [ROUND({ external: 3, ready: true })] }),
      'GET /api/assessments/a1/profile': jsonResponse({
        profile: liveProfile(),
        counts: { external: 3, self: 0, required: 3, ready: true },
      }),
    });

    await screen.findByText('Раунды 360°', { selector: 'h1' });

    const user = (await import('@testing-library/user-event')).default.setup();
    await user.click(screen.getByRole('button', { name: /Деструкторы/ }));

    expect(await screen.findByText('Критические зоны (деструкторы)')).toBeInTheDocument();
    expect(screen.queryByText(/Собрано \d+ из \d+ анкет/)).not.toBeInTheDocument();
  });

  it('в сетевом режиме не предлагает демо-опрос', async () => {
    renderApp({
      'GET /api/me': jsonResponse(LEADER),
      'GET /api/assessments': jsonResponse({ assessments: [] }),
    });

    await screen.findByText('Раунды 360°', { selector: 'h1' });
    // Анкеты заполняют приглашённые по своим ссылкам; локальный опрос здесь
    // писал бы в localStorage и ни на что не влиял.
    expect(screen.queryByRole('button', { name: /Опрос 360°/ })).not.toBeInTheDocument();
  });
});
