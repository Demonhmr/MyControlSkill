import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import RespondentSurveyScreen from './RespondentSurveyScreen.jsx';

function jsonResponse(body, status = 200) {
  return { ok: status >= 200 && status < 300, status, json: async () => body };
}

const READY = { role: 'peer', leaderName: 'lead@example.com', used: false, closed: false };

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('RespondentSurveyScreen', () => {
  it('показывает анкету и кого оценивают', async () => {
    fetch.mockResolvedValueOnce(jsonResponse(READY));
    render(<RespondentSurveyScreen token="тестовый-токен" />);

    expect(await screen.findByText(/lead@example.com/)).toBeInTheDocument();
    // Роль назначена приглашением, поэтому выбора роли на экране быть не должно.
    expect(screen.queryByText('Кто заполняет')).not.toBeInTheDocument();
    expect(screen.getByText('Как долго наблюдаете этого руководителя')).toBeInTheDocument();
    expect(screen.getByText('Ответы анонимны')).toBeInTheDocument();
  });

  it('сообщает, что ссылка недействительна', async () => {
    fetch.mockResolvedValueOnce(jsonResponse({ error: 'ссылка недействительна' }, 404));
    render(<RespondentSurveyScreen token="плохой" />);

    expect(await screen.findByText('Ссылка недействительна')).toBeInTheDocument();
  });

  it('сообщает, что по ссылке уже отвечали', async () => {
    fetch.mockResolvedValueOnce(jsonResponse({ ...READY, used: true }));
    render(<RespondentSurveyScreen token="использованный" />);

    expect(await screen.findByText('Анкета уже заполнена')).toBeInTheDocument();
    expect(screen.queryByText('Как долго наблюдаете этого руководителя')).not.toBeInTheDocument();
  });

  it('сообщает о закрытом раунде', async () => {
    fetch.mockResolvedValueOnce(jsonResponse({ ...READY, closed: true }));
    render(<RespondentSurveyScreen token="закрытый" />);

    expect(await screen.findByText('Сбор ответов завершён')).toBeInTheDocument();
  });

  it('не даёт отправить анкету без срока наблюдения', async () => {
    fetch.mockResolvedValueOnce(jsonResponse(READY));
    render(<RespondentSurveyScreen token="токен" />);

    const button = await screen.findByRole('button', { name: 'Отправить анкету' });
    expect(button).toBeDisabled();
    expect(screen.getByText(/Укажите срок наблюдения/)).toBeInTheDocument();
  });

  it('отправляет заполненную анкету и благодарит', async () => {
    const user = userEvent.setup();
    fetch.mockResolvedValueOnce(jsonResponse(READY));
    render(<RespondentSurveyScreen token="токен" />);

    await user.click(await screen.findByRole('button', { name: '> 1 года' }));
    // Первая шкала на экране относится к первому пункту первой компетенции.
    await user.click(screen.getAllByRole('button', { name: '4' })[0]);

    fetch.mockResolvedValueOnce(jsonResponse({ status: 'ok' }, 201));
    await user.click(screen.getByRole('button', { name: 'Отправить анкету' }));

    expect(await screen.findByText('Спасибо, анкета отправлена')).toBeInTheDocument();

    const [url, options] = fetch.mock.calls[1];
    expect(url).toBe('/api/survey/%D1%82%D0%BE%D0%BA%D0%B5%D0%BD');
    expect(options.method).toBe('POST');

    const sent = JSON.parse(options.body);
    expect(sent.tenure).toBe('gt12');
    expect(sent.answers).toHaveLength(1);
    expect(sent.answers[0]).toMatchObject({ kind: 'competency', itemIndex: 0, value: 4 });
    // Роль сервер берёт из приглашения, клиент её не присылает.
    expect(sent.role).toBeUndefined();
  });

  it('показывает отказ, если ссылку уже использовали', async () => {
    const user = userEvent.setup();
    fetch.mockResolvedValueOnce(jsonResponse(READY));
    render(<RespondentSurveyScreen token="токен" />);

    await user.click(await screen.findByRole('button', { name: '> 1 года' }));

    fetch.mockResolvedValueOnce(jsonResponse({ error: 'уже отправлена' }, 409));
    await user.click(screen.getByRole('button', { name: 'Отправить анкету' }));

    await waitFor(() => expect(screen.getByText('Анкета не отправлена')).toBeInTheDocument());
    // Форма остаётся на месте: ответы не должны пропасть вместе с отказом.
    expect(screen.getByText('Как долго наблюдаете этого руководителя')).toBeInTheDocument();
  });
});
