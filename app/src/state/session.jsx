import { createContext, useCallback, useContext, useEffect, useState } from 'react';
import { probeSession } from '../services/leaderApi.js';

const SessionCtx = createContext(null);

/**
 * Режим работы приложения и текущий руководитель.
 *
 * Режимов два. В local приложение живёт целиком в браузере — так работает
 * .exe, который раздают для показов; данные лежат в localStorage и никуда
 * не уходят. В server за приложением стоит бэкенд, и без входа оно ничего
 * не покажет.
 */
export function SessionProvider({ children }) {
  const [session, setSession] = useState({ status: 'detecting', mode: null, leader: null });

  const refresh = useCallback(async () => {
    try {
      const { mode, leader } = await probeSession();
      setSession({ status: 'ready', mode, leader });
    } catch {
      // Неопознанный сбой пробы — уходим в локальный режим: показать
      // демо лучше, чем пустой экран.
      setSession({ status: 'ready', mode: 'local', leader: null });
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return <SessionCtx.Provider value={{ ...session, refresh }}>{children}</SessionCtx.Provider>;
}

export function useSession() {
  const ctx = useContext(SessionCtx);
  if (!ctx) throw new Error('useSession must be used within SessionProvider');
  return ctx;
}
