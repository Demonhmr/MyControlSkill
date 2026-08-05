import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ConsentCard from './ConsentCard.jsx';
import { jsonResponse, routeFetch } from '../test/helpers.jsx';

function membership(granted) {
  return {
    org: { id: 'o1', name: 'ООО «Компас»' },
    role: 'leader',
    consentGranted: granted,
    consentAt: granted ? '2026-08-01T10:00:00.000Z' : null,
  };
}

function renderCard(routes) {
  vi.stubGlobal('fetch', routeFetch(routes));
  render(<ConsentCard />);
}

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('ConsentCard', () => {
  it('ничего не показывает вне организации', async () => {
    renderCard({ 'GET /api/me/org': jsonResponse({ error: 'нет организации' }, 404) });

    // Карточка про организацию бессмысленна там, где организации нет.
    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(screen.queryByText(/Организация/)).not.toBeInTheDocument();
  });

  it('по умолчанию показывает, что разрешения нет', async () => {
    renderCard({ 'GET /api/me/org': jsonResponse(membership(false)) });

    expect(await screen.findByText(/ООО «Компас»/)).toBeInTheDocument();
    expect(screen.getByText('не разрешено')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Разрешить показ HR-службе' })).toBeInTheDocument();
    // Важно, что человек понимает: без разрешения не видно и счётчиков.
    expect(screen.getByText(/не видит ни чисел/)).toBeInTheDocument();
  });

  it('выдаёт разрешение и показывает дату', async () => {
    const user = userEvent.setup();
    let granted = false;
    renderCard({
      'GET /api/me/org': () => jsonResponse(membership(granted)),
      'PUT /api/me/org/consent': () => {
        granted = true;
        return jsonResponse({ consentGranted: true, consentAt: '2026-08-01T10:00:00.000Z' });
      },
    });

    await user.click(await screen.findByRole('button', { name: 'Разрешить показ HR-службе' }));

    expect(await screen.findByText('разрешено')).toBeInTheDocument();
    expect(screen.getByText(/1 августа 2026/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Отозвать разрешение' })).toBeInTheDocument();
  });

  it('отзывает разрешение', async () => {
    const user = userEvent.setup();
    let granted = true;
    renderCard({
      'GET /api/me/org': () => jsonResponse(membership(granted)),
      'PUT /api/me/org/consent': () => {
        granted = false;
        return jsonResponse({ consentGranted: false, consentAt: null });
      },
    });

    await user.click(await screen.findByRole('button', { name: 'Отозвать разрешение' }));

    expect(await screen.findByText('не разрешено')).toBeInTheDocument();

    const [, options] = fetch.mock.calls.find(([url]) => url === '/api/me/org/consent');
    expect(JSON.parse(options.body)).toEqual({ granted: false });
  });

  it('сообщает, если решение не сохранилось', async () => {
    const user = userEvent.setup();
    renderCard({
      'GET /api/me/org': jsonResponse(membership(false)),
      'PUT /api/me/org/consent': jsonResponse({ error: 'сбой' }, 500),
    });

    await user.click(await screen.findByRole('button', { name: 'Разрешить показ HR-службе' }));

    // Молча оставить человека в уверенности, что он разрешил или отозвал, —
    // худший исход для согласия.
    await waitFor(() =>
      expect(screen.getByText('Не удалось сохранить решение. Попробуйте ещё раз.')).toBeInTheDocument()
    );
    expect(screen.getByText('не разрешено')).toBeInTheDocument();
  });
});
