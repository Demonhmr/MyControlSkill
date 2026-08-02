import { createContext, useContext, useEffect, useReducer } from 'react';
import { loadState, saveState } from '../services/api.js';

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

function init(base) {
  const saved = loadState();
  return saved ? { ...base, ...saved } : base;
}

function reducer(state, action) {
  switch (action.type) {
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

export function StoreProvider({ children }) {
  const [state, dispatch] = useReducer(reducer, initialState, init);

  useEffect(() => {
    const toSave = {};
    PERSISTED_KEYS.forEach((k) => (toSave[k] = state[k]));
    saveState(toSave);
  }, [state]);

  return <StoreCtx.Provider value={{ state, dispatch }}>{children}</StoreCtx.Provider>;
}

export function useStore() {
  const ctx = useContext(StoreCtx);
  if (!ctx) throw new Error('useStore must be used within StoreProvider');
  return ctx;
}

export function hasCriticalDestructor(state, destructors) {
  return destructors.some((d) => d.percentile < 10) && !state.destructorAcknowledged;
}
