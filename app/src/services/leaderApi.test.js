import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { probeSession, fetchProfile } from './leaderApi.js';

function response({ status = 200, contentType = 'application/json; charset=utf-8', body = {} } = {}) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: { get: (name) => (name === 'Content-Type' ? contentType : null) },
    json: async () => body,
  };
}

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn());
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('probeSession', () => {
  it('видит сервер и вошедшего руководителя', async () => {
    fetch.mockResolvedValueOnce(response({ body: { id: 'a1', email: 'lead@example.com' } }));

    await expect(probeSession()).resolves.toEqual({
      mode: 'server',
      leader: { id: 'a1', email: 'lead@example.com' },
    });
  });

  it('видит сервер без входа', async () => {
    fetch.mockResolvedValueOnce(response({ status: 401, body: { error: 'требуется вход' } }));

    await expect(probeSession()).resolves.toEqual({ mode: 'server', leader: null });
  });

  // Лаунчер на любой неизвестный путь отдаёт оболочку приложения с кодом 200,
  // поэтому режим различается по типу содержимого, а не по статусу.
  it('различает лаунчер по HTML вместо JSON', async () => {
    fetch.mockResolvedValueOnce(response({ contentType: 'text/html; charset=utf-8', body: {} }));

    await expect(probeSession()).resolves.toEqual({ mode: 'local', leader: null });
  });

  it('уходит в локальный режим, если сети нет', async () => {
    fetch.mockRejectedValueOnce(new TypeError('Failed to fetch'));

    await expect(probeSession()).resolves.toEqual({ mode: 'local', leader: null });
  });
});

describe('fetchProfile', () => {
  it('считает 423 состоянием сбора, а не ошибкой', async () => {
    fetch.mockResolvedValueOnce(
      response({ status: 423, body: { error: 'мало анкет', counts: { external: 2, required: 3 } } })
    );

    const result = await fetchProfile('a1');
    expect(result.ready).toBe(false);
    expect(result.profile).toBeNull();
    expect(result.counts.external).toBe(2);
  });

  it('отдаёт профиль после порога', async () => {
    fetch.mockResolvedValueOnce(
      response({ body: { profile: { ready: true }, counts: { external: 3, ready: true } } })
    );

    const result = await fetchProfile('a1');
    expect(result.ready).toBe(true);
    expect(result.profile).toEqual({ ready: true });
  });

  it('пробрасывает настоящую ошибку', async () => {
    fetch.mockResolvedValueOnce(response({ status: 500, body: { error: 'внутренняя ошибка' } }));

    await expect(fetchProfile('a1')).rejects.toMatchObject({ status: 500 });
  });
});
