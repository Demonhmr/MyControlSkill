import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { useStore } from './store.jsx';
import { useSession } from './session.jsx';
import { computeProfile } from '../logic/scoring.js';
import { adaptServerProfile } from '../logic/serverProfile.js';
import { fetchProfile, listAssessments } from '../services/leaderApi.js';

const ProfileCtx = createContext(null);

// Какой раунд смотрим — переживает перезагрузку страницы, иначе после каждого
// обновления приходилось бы выбирать заново.
const SELECTED_KEY = 'leadership-selected-assessment';

function readSelected() {
  try {
    return localStorage.getItem(SELECTED_KEY);
  } catch {
    return null;
  }
}

function writeSelected(id) {
  try {
    if (id) localStorage.setItem(SELECTED_KEY, id);
    else localStorage.removeItem(SELECTED_KEY);
  } catch {
    // Приватный режим браузера — переживём, просто выбор не запомнится.
  }
}

/**
 * Источник профиля.
 *
 * В локальном режиме профиль считается здесь же, из анкет в localStorage, и
 * ниже порога респондентов подставляет демонстрационные значения — это
 * витрина прототипа, и так и задумано.
 *
 * В сетевом режиме профиль приходит с сервера уже посчитанным. Считать его
 * на клиенте там нельзя: для этого пришлось бы отдать браузеру сырые анкеты,
 * а по ним видно, кто именно как ответил. Демонстрационных значений в этом
 * режиме нет вовсе — пока анкет мало, чисел не показываем.
 */
export function ProfileProvider({ children }) {
  const { mode } = useSession();
  const { state } = useStore();

  const localProfile = useMemo(() => computeProfile(state.responses), [state.responses]);

  const [remote, setRemote] = useState({
    status: 'loading',
    profile: null,
    counts: null,
    assessments: [],
    assessmentId: null,
    error: null,
  });

  const load = useCallback(async (preferredId) => {
    setRemote((prev) => ({ ...prev, status: 'loading', error: null }));
    try {
      const { assessments } = await listAssessments();
      const list = assessments ?? [];
      if (list.length === 0) {
        setRemote({
          status: 'no-assessments',
          profile: null,
          counts: null,
          assessments: [],
          assessmentId: null,
          error: null,
        });
        return;
      }

      // Список приходит свежими первыми, поэтому запасной вариант — верхний.
      const wanted = preferredId ?? readSelected();
      const chosen = list.find((a) => a.id === wanted) ?? list[0];
      writeSelected(chosen.id);

      const result = await fetchProfile(chosen.id);
      setRemote({
        status: result.ready ? 'ready' : 'collecting',
        profile: result.ready ? adaptServerProfile(result.profile) : null,
        counts: result.counts,
        assessments: list,
        assessmentId: chosen.id,
        error: null,
      });
    } catch (err) {
      setRemote((prev) => ({ ...prev, status: 'error', error: err }));
    }
  }, []);

  useEffect(() => {
    if (mode === 'server') load();
  }, [mode, load]);

  const value =
    mode === 'server'
      ? {
          source: 'server',
          profile: remote.profile,
          status: remote.status,
          counts: remote.counts,
          assessments: remote.assessments,
          assessmentId: remote.assessmentId,
          error: remote.error,
          select: (id) => load(id),
          reload: () => load(remote.assessmentId),
        }
      : {
          source: 'local',
          profile: localProfile,
          status: 'ready',
          counts: null,
          assessments: [],
          assessmentId: null,
          error: null,
          select: () => {},
          reload: () => {},
        };

  return <ProfileCtx.Provider value={value}>{children}</ProfileCtx.Provider>;
}

export function useProfileSource() {
  const ctx = useContext(ProfileCtx);
  if (!ctx) throw new Error('useProfileSource must be used within ProfileProvider');
  return ctx;
}
