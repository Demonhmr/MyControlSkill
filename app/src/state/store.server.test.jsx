import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SessionProvider } from './session.jsx';
import { StoreProvider, useStore } from './store.jsx';
import { jsonResponse, routeFetch } from '../test/helpers.jsx';

const LEADER = { id: 'l1', email: 'lead@example.com', name: '' };

// Маленькая витрина стора: экраны сюда тащить незачем, проверяется сам стор.
function Harness() {
  const { state, dispatch, addReflection, hydrated } = useStore();
  return (
    <div>
      <span data-testid="hydrated">{String(hydrated)}</span>
      <span data-testid="growth">{state.growthPoint ?? '—'}</span>
      <span data-testid="reflections">{state.reflections.map((r) => r.text).join('|') || '—'}</span>
      <button onClick={() => dispatch({ type: 'SET_GROWTH_POINT', code: 'COM' })}>Выбрать COM</button>
      <button onClick={() => addReflection('COM', 'Провёл разговор')}>Записать</button>
    </div>
  );
}

function renderStore(routes) {
  vi.stubGlobal('fetch', routeFetch(routes));
  render(
    <SessionProvider>
      <StoreProvider>
        <Harness />
      </StoreProvider>
    </SessionProvider>
  );
}

function putCalls() {
  return fetch.mock.calls.filter(([url, opts]) => url === '/api/state' && opts?.method === 'PUT');
}

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
});

describe('стор в сетевом режиме', () => {
  it('поднимает состояние и записи с сервера', async () => {
    renderStore({
      'GET /api/me': jsonResponse(LEADER),
      'GET /api/state': jsonResponse({
        state: { growthPoint: 'REL', destructorAcknowledged: true },
        reflections: [{ code: 'REL', text: 'Старая запись', createdAt: '2026-08-01T10:00:00.000Z' }],
      }),
      'PUT /api/state': jsonResponse(null, 204),
    });

    // Ждём сами данные, а не флаг: пока режим ещё определяется, флаг
    // разрешающий, и проверка на него прошла бы вхолостую.
    await waitFor(() => expect(screen.getByTestId('growth')).toHaveTextContent('REL'));
    expect(screen.getByTestId('reflections')).toHaveTextContent('Старая запись');
    expect(screen.getByTestId('hydrated')).toHaveTextContent('true');
  });

  it('не пишет на сервер до того, как прочитал оттуда', async () => {
    let releaseGet;
    const pending = new Promise((resolve) => {
      releaseGet = resolve;
    });

    renderStore({
      'GET /api/me': jsonResponse(LEADER),
      'GET /api/state': () => pending,
      'PUT /api/state': jsonResponse(null, 204),
    });

    // Пока чтение висит, состояние по умолчанию не должно уехать на сервер:
    // оно затёрло бы то, что там лежит.
    await new Promise((r) => setTimeout(r, 1200));
    expect(putCalls()).toHaveLength(0);

    releaseGet(jsonResponse({ state: { growthPoint: 'REL' }, reflections: [] }));
    await waitFor(() => expect(screen.getByTestId('growth')).toHaveTextContent('REL'));
  });

  it('не пишет на сервер, если чтение не удалось', async () => {
    renderStore({
      'GET /api/me': jsonResponse(LEADER),
      'GET /api/state': jsonResponse({ error: 'внутренняя ошибка' }, 500),
      'PUT /api/state': jsonResponse(null, 204),
    });

    const user = userEvent.setup();
    await waitFor(() => expect(screen.getByTestId('hydrated')).toHaveTextContent('false'));
    await user.click(screen.getByRole('button', { name: 'Выбрать COM' }));

    // Правки видно на экране, но затирать ими неизвестное состояние нельзя.
    expect(screen.getByTestId('growth')).toHaveTextContent('COM');
    await new Promise((r) => setTimeout(r, 1200));
    expect(putCalls()).toHaveLength(0);
  });

  it('сохраняет отметки одним отложенным запросом', async () => {
    renderStore({
      'GET /api/me': jsonResponse(LEADER),
      'GET /api/state': jsonResponse({ state: {}, reflections: [] }),
      'PUT /api/state': jsonResponse(null, 204),
    });

    const user = userEvent.setup();
    await waitFor(() => expect(screen.getByTestId('hydrated')).toHaveTextContent('true'));
    await user.click(screen.getByRole('button', { name: 'Выбрать COM' }));

    await waitFor(() => expect(putCalls().length).toBeGreaterThan(0), { timeout: 3000 });

    const [, options] = putCalls().at(-1);
    const sent = JSON.parse(options.body).state;
    expect(sent.growthPoint).toBe('COM');
    // Анкеты в сетевом режиме живут в своих таблицах, записи — в своих:
    // в рабочем состоянии им не место.
    expect(sent.responses).toBeUndefined();
    expect(sent.reflections).toBeUndefined();
  });

  it('показывает запись только после сохранения на сервере', async () => {
    renderStore({
      'GET /api/me': jsonResponse(LEADER),
      'GET /api/state': jsonResponse({ state: {}, reflections: [] }),
      'PUT /api/state': jsonResponse(null, 204),
      'POST /api/reflections': jsonResponse(
        { code: 'COM', text: 'Провёл разговор', createdAt: '2026-08-04T10:00:00.000Z' },
        201
      ),
    });

    const user = userEvent.setup();
    await waitFor(() => expect(screen.getByTestId('hydrated')).toHaveTextContent('true'));
    await user.click(screen.getByRole('button', { name: 'Записать' }));

    await waitFor(() => expect(screen.getByTestId('reflections')).toHaveTextContent('Провёл разговор'));
  });

  it('не показывает запись, если сервер её не принял', async () => {
    renderStore({
      'GET /api/me': jsonResponse(LEADER),
      'GET /api/state': jsonResponse({ state: {}, reflections: [] }),
      'PUT /api/state': jsonResponse(null, 204),
      'POST /api/reflections': jsonResponse({ error: 'не удалось' }, 500),
    });

    const user = userEvent.setup();
    await waitFor(() => expect(screen.getByTestId('hydrated')).toHaveTextContent('true'));
    await user.click(screen.getByRole('button', { name: 'Записать' }));

    // Иначе запись осталась бы на экране и нигде больше — худший исход:
    // руководитель уверен, что практика зафиксирована.
    await new Promise((r) => setTimeout(r, 300));
    expect(screen.getByTestId('reflections')).toHaveTextContent('—');
  });
});

describe('стор в локальном режиме', () => {
  it('поднимает состояние из localStorage и на сервер не ходит', async () => {
    localStorage.setItem(
      'leadership-app-state-v1',
      JSON.stringify({ growthPoint: 'INT', reflections: [], responses: [] })
    );

    renderStore({ 'GET /api/me': { ok: true, status: 200, headers: { get: () => 'text/html' }, json: async () => ({}) } });

    // Синхронно, без ожидания: демо не должно мигать значениями по умолчанию.
    expect(screen.getByTestId('growth')).toHaveTextContent('INT');
    expect(screen.getByTestId('hydrated')).toHaveTextContent('true');

    await new Promise((r) => setTimeout(r, 1200));
    expect(putCalls()).toHaveLength(0);
  });
});
