import { useMemo } from 'react';
import { useStore } from './store.jsx';
import { computeProfile } from '../logic/scoring';

export function useProfile() {
  const { state } = useStore();
  return useMemo(() => computeProfile(state.responses), [state.responses]);
}
