import { vi } from 'vitest';
import { SessionProvider } from '../state/session.jsx';
import { StoreProvider } from '../state/store.jsx';
import { ProfileProvider } from '../state/profile.jsx';

/** Полный набор провайдеров приложения — для экранов, которым нужен профиль. */
export function AllProviders({ children }) {
  return (
    <SessionProvider>
      <StoreProvider>
        <ProfileProvider>{children}</ProfileProvider>
      </StoreProvider>
    </SessionProvider>
  );
}

export function jsonResponse(body, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: { get: () => 'application/json' },
    json: async () => body,
  };
}

export function htmlResponse() {
  return {
    ok: true,
    status: 200,
    headers: { get: () => 'text/html; charset=utf-8' },
    json: async () => {
      throw new SyntaxError('это HTML');
    },
  };
}

/**
 * Заглушка fetch по маршрутам, а не по очереди вызовов.
 *
 * Порядок запросов зависит от того, что и когда решит подгрузить React;
 * привязка к очерёдности делает тесты хрупкими там, где поведение на самом
 * деле не изменилось.
 *
 * Ключ — «МЕТОД /путь», значение — ответ или функция, его возвращающая.
 */
export function routeFetch(routes) {
  return vi.fn(async (url, options = {}) => {
    const key = `${options.method ?? 'GET'} ${url}`;
    const route = routes[key];
    if (route === undefined) {
      throw new Error(`нет заглушки для ${key}`);
    }
    return typeof route === 'function' ? route() : route;
  });
}
