import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import HROverviewScreen from './HROverviewScreen.jsx';
import { jsonResponse, routeFetch } from '../test/helpers.jsx';

function leaderRow({ email, ready, external = 0, destructors = [], strengths = [], hasCritical = false, role = 'leader' }) {
  return {
    leaderId: `id-${email}`,
    name: email,
    email,
    role,
    ready,
    counts: { external, self: 0, required: 3, ready },
    destructors,
    strengths,
    hasCritical,
  };
}

// Полный набор деструкторов: тепловая карта рисует все десять столбцов.
const ALL_DESTRUCTORS = ['d1', 'd2', 'd3', 'd4', 'd5', 'd6', 'd7', 'd8', 'd9', 'd10'].map((code, i) => ({
  code,
  percentile: code === 'd3' ? 7 : 40 + i,
}));

function renderScreen(routes) {
  vi.stubGlobal('fetch', routeFetch(routes));
  render(<HROverviewScreen />);
}

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('HROverviewScreen', () => {
  it('предлагает создать организацию, если её нет', async () => {
    renderScreen({ 'GET /api/hr/overview': jsonResponse({ error: 'нет организации' }, 404) });

    expect(await screen.findByLabelText('Название организации')).toBeInTheDocument();
  });

  it('создаёт организацию и перечитывает сводку', async () => {
    const user = userEvent.setup();
    let created = false;
    renderScreen({
      'GET /api/hr/overview': () =>
        created
          ? jsonResponse({ org: { id: 'o1', name: 'Компас' }, leaders: [] })
          : jsonResponse({ error: 'нет организации' }, 404),
      'POST /api/hr/org': () => {
        created = true;
        return jsonResponse({ id: 'o1', name: 'Компас' }, 201);
      },
    });

    await user.type(await screen.findByLabelText('Название организации'), 'Компас');
    await user.click(screen.getByRole('button', { name: 'Создать организацию' }));

    expect(await screen.findByText(/HR-дашборд: Компас/)).toBeInTheDocument();
  });

  it('не показывает сводку тому, кто не HR', async () => {
    renderScreen({ 'GET /api/hr/overview': jsonResponse({ error: 'только HR' }, 403) });

    expect(await screen.findByText('Сводка доступна только роли HR')).toBeInTheDocument();
    expect(screen.queryByLabelText('Название организации')).not.toBeInTheDocument();
  });

  // Главное свойство экрана: пока анкет мало, чисел по человеку нет вовсе.
  it('разделяет готовых руководителей и тех, по кому идёт сбор', async () => {
    renderScreen({
      'GET /api/hr/overview': jsonResponse({
        org: { id: 'o1', name: 'Компас' },
        leaders: [
          leaderRow({
            email: 'ready@example.com',
            ready: true,
            external: 4,
            destructors: ALL_DESTRUCTORS,
            strengths: [{ code: 'COM', percentile: 93 }],
            hasCritical: true,
          }),
          leaderRow({ email: 'waiting@example.com', ready: false, external: 1 }),
        ],
      }),
    });

    expect(await screen.findByText('Сбор ещё идёт')).toBeInTheDocument();
    expect(screen.getByText('1 из 3')).toBeInTheDocument();

    // У готового — числа и признак критической зоны.
    expect(screen.getByText(/Коммуникация · 93/)).toBeInTheDocument();
    expect(screen.getByText('⚠ есть критическая зона')).toBeInTheDocument();
  });

  it('сообщает, когда считать ещё нечего', async () => {
    renderScreen({
      'GET /api/hr/overview': jsonResponse({
        org: { id: 'o1', name: 'Компас' },
        leaders: [leaderRow({ email: 'waiting@example.com', ready: false, external: 2 })],
      }),
    });

    expect(await screen.findByText('Пока считать нечего')).toBeInTheDocument();
    expect(screen.queryByText('Тепловая карта деструкторов')).not.toBeInTheDocument();
  });

  it('добавляет участника и объясняет отказ', async () => {
    const user = userEvent.setup();
    renderScreen({
      'GET /api/hr/overview': jsonResponse({ org: { id: 'o1', name: 'Компас' }, leaders: [] }),
      'POST /api/hr/members': jsonResponse({ error: 'уже в другой организации' }, 409),
    });

    await user.type(await screen.findByLabelText('Почта руководителя'), 'lead@example.com');
    await user.click(screen.getByRole('button', { name: 'Добавить' }));

    await waitFor(() =>
      expect(screen.getByText('Этот человек уже состоит в другой организации.')).toBeInTheDocument()
    );
  });
});
