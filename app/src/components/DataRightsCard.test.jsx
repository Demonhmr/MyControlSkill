import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import DataRightsCard from './DataRightsCard.jsx';
import { SessionProvider } from '../state/session.jsx';
import { jsonResponse, routeFetch } from '../test/helpers.jsx';

const LEADER = { id: 'l1', email: 'lead@example.com', name: '' };

function renderCard(routes) {
  vi.stubGlobal('fetch', routeFetch({ 'GET /api/me': jsonResponse(LEADER), ...routes }));
  render(
    <SessionProvider>
      <DataRightsCard />
    </SessionProvider>
  );
}

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('DataRightsCard', () => {
  // Выгрузка идёт обычной ссылкой: сервер отдаёт файл с Content-Disposition,
  // и браузер сохраняет его сам.
  it('даёт скачать выгрузку ссылкой', async () => {
    renderCard({});

    const link = await screen.findByRole('link', { name: 'Скачать выгрузку' });
    expect(link).toHaveAttribute('href', '/api/me/export');
  });

  it('объясняет, чего в выгрузке нет', async () => {
    renderCard({});

    expect(await screen.findByText(/Отдельных ответов респондентов в ней нет/)).toBeInTheDocument();
  });

  it('не удаляет аккаунт с первого нажатия', async () => {
    const user = userEvent.setup();
    renderCard({});

    await user.click(await screen.findByRole('button', { name: 'Удалить аккаунт' }));

    // Сначала предупреждение и ввод адреса, а не удаление.
    expect(screen.getByText('Удаление необратимо')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Удалить навсегда' })).toBeDisabled();
    expect(fetch).not.toHaveBeenCalledWith('/api/me', expect.objectContaining({ method: 'DELETE' }));
  });

  it('удаляет аккаунт после подтверждения адресом', async () => {
    const user = userEvent.setup();
    let deleted = false;
    renderCard({
      'DELETE /api/me': () => {
        deleted = true;
        return jsonResponse(null, 204);
      },
    });

    await user.click(await screen.findByRole('button', { name: 'Удалить аккаунт' }));
    await user.type(screen.getByLabelText('Подтверждение адресом почты'), 'lead@example.com');
    await user.click(screen.getByRole('button', { name: 'Удалить навсегда' }));

    await waitFor(() => expect(deleted).toBe(true));

    const [, options] = fetch.mock.calls.find(([url, o]) => url === '/api/me' && o?.method === 'DELETE');
    expect(JSON.parse(options.body)).toEqual({ confirmEmail: 'lead@example.com' });
  });

  it('объясняет несовпадение адреса', async () => {
    const user = userEvent.setup();
    renderCard({ 'DELETE /api/me': jsonResponse({ error: 'не совпадает' }, 400) });

    await user.click(await screen.findByRole('button', { name: 'Удалить аккаунт' }));
    await user.type(screen.getByLabelText('Подтверждение адресом почты'), 'other@example.com');
    await user.click(screen.getByRole('button', { name: 'Удалить навсегда' }));

    await waitFor(() =>
      expect(screen.getByText('Адрес не совпадает с вашим. Введите его точно.')).toBeInTheDocument()
    );
  });

  it('даёт передумать', async () => {
    const user = userEvent.setup();
    renderCard({});

    await user.click(await screen.findByRole('button', { name: 'Удалить аккаунт' }));
    await user.click(screen.getByRole('button', { name: 'Отмена' }));

    expect(screen.queryByText('Удаление необратимо')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Удалить аккаунт' })).toBeInTheDocument();
  });
});
