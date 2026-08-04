// Общий разбор ответов сервера.

export class ApiError extends Error {
  constructor(message, status) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

/** Ответ пришёл от бэкенда, а не от оболочки SPA. */
export function isJSON(response) {
  return (response.headers?.get?.('Content-Type') ?? '').includes('json');
}

export async function parseResponse(response) {
  let body = null;
  try {
    body = await response.json();
  } catch {
    // Пустое тело (204) или не-JSON — сообщение возьмём из статуса.
  }
  if (!response.ok) {
    throw new ApiError(body?.error ?? 'Сервер вернул ошибку', response.status);
  }
  return body;
}

/** Запрос к API с телом в JSON. */
export async function request(path, { method = 'GET', body } = {}) {
  const response = await fetch(path, {
    method,
    headers: {
      Accept: 'application/json',
      ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  return parseResponse(response);
}
