import { createContext, useCallback, useContext, useEffect, useReducer, useRef, useState } from 'react';
import { loadState, saveState } from '../services/api.js';
import { useSession } from './session.jsx';
import {
  addReflection as postReflection,
  fetchLeaderState,
  saveLeaderState,
} from '../services/leaderApi.js';

const initialState = {
  screen: 'survey',
  destructorAcknowledged: false,
  growthPoint: null,
  interest: {},
  needs: {},
  trainerScenarioCode: null,
  reflections: [],
  responses: [],
};

const PERSISTED_KEYS = ['responses', 'destructorAcknowledged', 'growthPoint', 'interest', 'needs', 'reflections'];

// На сервер уходит только то, что руководитель наотмечал сам.
//
// responses здесь нет: в сетевом режиме анкеты приходят по ссылкам и живут в
// своих таблицах, а этот массив — принадлежность демо-режима. reflections
// тоже нет: у них свой эндпоинт, они временной ряд, а не поле формы.
const SERVER_KEYS = ['destructorAcknowledged', 'growthPoint', 'interest', 'needs', 'trainerScenarioCode'];

// Сохранение откладывается: отметка интереса меняет состояние на каждый
// клик, и слать запрос на каждый было бы расточительно.
const SAVE_DELAY_MS = 800;

function pick(state, keys) {
  const out = {};
  keys.forEach((k) => (out[k] = state[k]));
  return out;
}

function init(base) {
  const saved = loadState();
  return saved ? { ...base, ...saved } : base;
}

function reducer(state, action) {
  switch (action.type) {
    case 'HYDRATE':
      return { ...state, ...action.state };
    case 'SET_SCREEN':
      return { ...state, screen: action.screen };
    case 'ACK_DESTRUCTOR':
      return { ...state, destructorAcknowledged: true };
    case 'TOGGLE_INTEREST':
      return { ...state, interest: { ...state.interest, [action.code]: !state.interest[action.code] } };
    case 'TOGGLE_NEED':
      return { ...state, needs: { ...state.needs, [action.id]: !state.needs[action.id] } };
    case 'SET_GROWTH_POINT':
      return { ...state, growthPoint: action.code, trainerScenarioCode: action.code };
    case 'SET_TRAINER_SCENARIO':
      return { ...state, trainerScenarioCode: action.code };
    case 'ADD_REFLECTION':
      return { ...state, reflections: [{ date: action.date, code: action.code, text: action.text }, ...state.reflections] };
    case 'ADD_RESPONSE':
      return { ...state, responses: [...state.responses, action.response] };
    case 'RESET_DEMO':
      return { ...initialState, responses: [] };
    default:
      return state;
  }
}

const StoreCtx = createContext(null);

function formatDate(iso) {
  return new Date(iso).toLocaleDateString('ru-RU');
}

export function StoreProvider({ children }) {
  const { mode } = useSession();
  const isServer = mode === 'server';

  // В локальном режиме состояние поднимается из localStorage синхронно,
  // чтобы экран не мигал значениями по умолчанию. С сервера синхронно
  // не прочитать, поэтому там стартуем с пустого и догружаем.
  const [state, dispatch] = useReducer(reducer, initialState, (base) => (isServer ? base : init(base)));

  // Пока состояние не загружено, сохранять нельзя: пустое состояние по
  // умолчанию затёрло бы то, что лежит на сервере.
  //
  // Флаг выводится из режима, а не хранится готовым: режим выясняется
  // асинхронно, и запомненное при монтировании значение осталось бы
  // разрешающим, когда приложение уже переключилось на сервер.
  const [serverHydrated, setServerHydrated] = useState(false);
  const hydrated = isServer ? serverHydrated : true;
  const [persistError, setPersistError] = useState(null);

  useEffect(() => {
    if (!isServer) return undefined;
    let cancelled = false;

    fetchLeaderState()
      .then(({ state: remote, reflections }) => {
        if (cancelled) return;
        dispatch({
          type: 'HYDRATE',
          state: {
            ...(remote ?? {}),
            reflections: (reflections ?? []).map((r) => ({
              date: formatDate(r.createdAt),
              code: r.code,
              text: r.text,
            })),
          },
        });
        setServerHydrated(true);
      })
      .catch((err) => {
        if (cancelled) return;
        // hydrated намеренно остаётся false: раз прочитать не удалось,
        // писать поверх тем более нельзя.
        setPersistError(err);
      });

    return () => {
      cancelled = true;
    };
  }, [isServer]);

  useEffect(() => {
    if (!hydrated) return undefined;

    if (!isServer) {
      saveState(pick(state, PERSISTED_KEYS));
      return undefined;
    }

    const timer = setTimeout(() => {
      saveLeaderState(pick(state, SERVER_KEYS))
        .then(() => setPersistError(null))
        .catch(setPersistError);
    }, SAVE_DELAY_MS);
    return () => clearTimeout(timer);
  }, [state, hydrated, isServer]);

  // Запись из тренажёра сначала уходит на сервер и только потом попадает в
  // состояние: иначе при сбое она осталась бы на экране, но нигде больше.
  const savingRef = useRef(false);
  const addReflection = useCallback(
    async (code, text) => {
      if (!isServer) {
        dispatch({ type: 'ADD_REFLECTION', date: new Date().toLocaleDateString('ru-RU'), code, text });
        return;
      }
      if (savingRef.current) return;
      savingRef.current = true;
      try {
        const saved = await postReflection(code, text);
        dispatch({ type: 'ADD_REFLECTION', date: formatDate(saved.createdAt), code: saved.code, text: saved.text });
        setPersistError(null);
      } catch (err) {
        setPersistError(err);
      } finally {
        savingRef.current = false;
      }
    },
    [isServer]
  );

  return (
    <StoreCtx.Provider value={{ state, dispatch, addReflection, hydrated, persistError }}>
      {children}
    </StoreCtx.Provider>
  );
}

export function useStore() {
  const ctx = useContext(StoreCtx);
  if (!ctx) throw new Error('useStore must be used within StoreProvider');
  return ctx;
}

export function hasCriticalDestructor(state, destructors) {
  return destructors.some((d) => d.percentile < 10) && !state.destructorAcknowledged;
}
