import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import AssessmentsScreen from './AssessmentsScreen.jsx';

function jsonResponse(body, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: { get: () => 'application/json' },
    json: async () => body,
  };
}

const ROUND = {
  id: 'a1',
  title: 'Пилот, август',
  createdAt: '2026-08-01T10:00:00.000Z',
  closedAt: null,
  counts: { external: 1, self: 0, required: 3, ready: false },
};

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('AssessmentsScreen', () => {
  it('показывает раунды со счётчиком собранных анкет', async () => {
    fetch.mockResolvedValueOnce(jsonResponse({ assessments: [ROUND] }));
    render(<AssessmentsScreen />);

    expect(await screen.findByText('Пилот, август')).toBeInTheDocument();
    expect(screen.getByText('1 из 3')).toBeInTheDocument();
  });

  it('сообщает, когда раундов нет', async () => {
    fetch.mockResolvedValueOnce(jsonResponse({ assessments: [] }));
    render(<AssessmentsScreen />);

    expect(await screen.findByText('Пока ни одного раунда.')).toBeInTheDocument();
  });

  it('открывает раунд и объясняет, почему профиль не считается', async () => {
    const user = userEvent.setup();
    fetch.mockResolvedValueOnce(jsonResponse({ assessments: [ROUND] }));
    render(<AssessmentsScreen />);

    fetch.mockResolvedValueOnce(jsonResponse({ assessment: ROUND, invites: [] }));
    await user.click(await screen.findByRole('button', { name: 'Пилот, август' }));

    expect(await screen.findByText('Профиль пока не считается')).toBeInTheDocument();
    expect(screen.getByText('Пока никого не пригласили.')).toBeInTheDocument();
  });

  it('выдаёт приглашение и показывает ссылку', async () => {
    const user = userEvent.setup();
    fetch.mockResolvedValueOnce(jsonResponse({ assessments: [ROUND] }));
    render(<AssessmentsScreen />);

    fetch.mockResolvedValueOnce(jsonResponse({ assessment: ROUND, invites: [] }));
    await user.click(await screen.findByRole('button', { name: 'Пилот, август' }));
    await screen.findByText('Пригласить респондента');

    const invite = {
      id: 'i1',
      role: 'peer',
      email: 'peer@example.com',
      createdAt: '2026-08-02T10:00:00.000Z',
      usedAt: null,
    };
    // Ответ на выдачу приглашения, затем перезагрузка списка и раунда.
    fetch
      .mockResolvedValueOnce(jsonResponse({ invite, link: 'https://example.com/s/ТОКЕН' }, 201))
      .mockResolvedValueOnce(jsonResponse({ assessments: [ROUND] }))
      .mockResolvedValueOnce(jsonResponse({ assessment: ROUND, invites: [invite] }));

    await user.type(screen.getByLabelText('Почта респондента'), 'peer@example.com');
    await user.click(screen.getByRole('button', { name: 'Выдать ссылку' }));

    // Ссылка показывается один раз: сервер хранит только её хэш.
    expect(await screen.findByText('https://example.com/s/ТОКЕН')).toBeInTheDocument();
    expect(screen.getByText('ждём ответа')).toBeInTheDocument();
  });

  it('объясняет отказ выдать приглашение в закрытый раунд', async () => {
    const user = userEvent.setup();
    fetch.mockResolvedValueOnce(jsonResponse({ assessments: [ROUND] }));
    render(<AssessmentsScreen />);

    fetch.mockResolvedValueOnce(jsonResponse({ assessment: ROUND, invites: [] }));
    await user.click(await screen.findByRole('button', { name: 'Пилот, август' }));
    await screen.findByText('Пригласить респондента');

    fetch.mockResolvedValueOnce(jsonResponse({ error: 'раунд закрыт' }, 409));
    await user.click(screen.getByRole('button', { name: 'Выдать ссылку' }));

    await waitFor(() =>
      expect(screen.getByText('Раунд закрыт — приглашения больше не выдаются.')).toBeInTheDocument()
    );
  });

  it('в закрытом раунде не предлагает приглашать', async () => {
    const user = userEvent.setup();
    const closed = { ...ROUND, closedAt: '2026-08-03T10:00:00.000Z' };
    fetch.mockResolvedValueOnce(jsonResponse({ assessments: [closed] }));
    render(<AssessmentsScreen />);

    fetch.mockResolvedValueOnce(jsonResponse({ assessment: closed, invites: [] }));
    await user.click(await screen.findByRole('button', { name: 'Пилот, август' }));

    await screen.findByText('Приглашения');
    expect(screen.queryByText('Пригласить респондента')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Закрыть раунд' })).not.toBeInTheDocument();
  });
});
