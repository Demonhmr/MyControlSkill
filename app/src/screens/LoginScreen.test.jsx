import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import LoginScreen from './LoginScreen.jsx';

function jsonResponse(body, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: { get: () => 'application/json' },
    json: async () => body,
  };
}

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  window.history.replaceState({}, '', '/');
});

describe('LoginScreen', () => {
  it('не даёт запросить ссылку без почты', () => {
    render(<LoginScreen />);
    expect(screen.getByRole('button', { name: 'Прислать ссылку' })).toBeDisabled();
  });

  it('запрашивает ссылку и подтверждает отправку', async () => {
    const user = userEvent.setup();
    render(<LoginScreen />);

    await user.type(screen.getByLabelText('Рабочая почта'), 'lead@example.com');
    fetch.mockResolvedValueOnce(jsonResponse({ status: 'ok' }, 202));
    await user.click(screen.getByRole('button', { name: 'Прислать ссылку' }));

    expect(await screen.findByText('Ссылка отправлена')).toBeInTheDocument();

    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe('/api/auth/login');
    expect(JSON.parse(options.body)).toEqual({ email: 'lead@example.com' });
  });

  // Явно кривой адрес до сервера не доедет: поле типа email браузер
  // проверяет сам. Проверяем то, что проверить может только сервер, —
  // адрес правильной формы, который он всё равно отвергает.
  it('показывает ошибку, когда адрес отвергает сервер', async () => {
    const user = userEvent.setup();
    render(<LoginScreen />);

    await user.type(screen.getByLabelText('Рабочая почта'), 'lead@example');
    fetch.mockResolvedValueOnce(jsonResponse({ error: 'некорректный адрес' }, 400));
    await user.click(screen.getByRole('button', { name: 'Прислать ссылку' }));

    await waitFor(() => expect(screen.getByText('Проверьте адрес почты.')).toBeInTheDocument());
  });

  it('не отправляет запрос, если браузер забраковал адрес', async () => {
    const user = userEvent.setup();
    render(<LoginScreen />);

    await user.type(screen.getByLabelText('Рабочая почта'), 'не почта');
    await user.click(screen.getByRole('button', { name: 'Прислать ссылку' }));

    expect(fetch).not.toHaveBeenCalled();
  });

  // Сервер возвращает на главную с пометкой о причине отказа — экран должен
  // объяснить её словами, а не молчать.
  it('объясняет причину неудачного перехода по ссылке', async () => {
    window.history.replaceState({}, '', '/?login_error=expired');
    render(<LoginScreen />);

    expect(screen.getByText(/Срок действия ссылки истёк/)).toBeInTheDocument();
  });

  it('не пугает пользователя при обычном заходе', () => {
    render(<LoginScreen />);
    expect(screen.queryByText('Не вышло войти')).not.toBeInTheDocument();
  });
});
