import { StoreProvider, useStore } from './state/store.jsx';
import { SessionProvider, useSession } from './state/session.jsx';
import { ProfileProvider, useProfileSource } from './state/profile.jsx';
import TopNav from './components/TopNav.jsx';
import { Banner, Card } from './components/ui.jsx';
import Survey360Screen from './screens/Survey360Screen.jsx';
import DestructorsScreen from './screens/DestructorsScreen.jsx';
import StrengthMapScreen from './screens/StrengthMapScreen.jsx';
import GrowthPointScreen from './screens/GrowthPointScreen.jsx';
import PlanScreen from './screens/PlanScreen.jsx';
import TrainerScreen from './screens/TrainerScreen.jsx';
import PulseScreen from './screens/PulseScreen.jsx';
import HRDashboardScreen from './screens/HRDashboardScreen.jsx';
import AssessmentsScreen from './screens/AssessmentsScreen.jsx';
import LoginScreen from './screens/LoginScreen.jsx';
import ProfileGateScreen from './screens/ProfileGateScreen.jsx';

const SCREENS = {
  survey: Survey360Screen,
  destructors: DestructorsScreen,
  strength: StrengthMapScreen,
  growth: GrowthPointScreen,
  plan: PlanScreen,
  trainer: TrainerScreen,
  pulse: PulseScreen,
  hr: HRDashboardScreen,
  rounds: AssessmentsScreen,
};

// Экраны, которые целиком построены на числах профиля. Без готового расчёта
// им нечего показать, кроме демо-значений, а выдавать их за результат замера
// нельзя.
const PROFILE_SCREENS = new Set(['destructors', 'strength', 'growth', 'plan', 'pulse']);

function Shell() {
  const { state, persistError } = useStore();
  const { mode } = useSession();
  const { profile } = useProfileSource();

  // «Опрос 360°» — экран демо-режима: он пишет анкеты в localStorage, и в
  // сетевом режиме они ни на что не влияют. Там вход в приложение начинается
  // с раундов.
  const screen = mode === 'server' && state.screen === 'survey' ? 'rounds' : state.screen;

  const needsProfile = mode === 'server' && PROFILE_SCREENS.has(screen);
  const Screen = needsProfile && !profile ? ProfileGateScreen : SCREENS[screen] ?? Survey360Screen;

  return (
    <div className="app viz-root">
      <TopNav />
      {/* Молча терять план развития нельзя: без этого сообщения руководитель
          решит, что всё сохранилось, и обнаружит пропажу на другом устройстве. */}
      {persistError && (
        <Banner title="Изменения не сохраняются">
          {persistError.status === 401
            ? 'Сессия истекла — войдите заново, иначе отметки останутся только в этом окне.'
            : 'Связь с сервером потеряна. Отметки останутся только в этом окне, пока она не восстановится.'}
        </Banner>
      )}
      <Screen />
    </div>
  );
}

export default function App() {
  return (
    <SessionProvider>
      <Gate />
    </SessionProvider>
  );
}

/**
 * Решает, что показать: демо, вход или само приложение.
 *
 * В локальном режиме входа нет вовсе — .exe раздают для показов, и требовать
 * там аккаунт бессмысленно.
 */
function Gate() {
  const { status, mode, leader } = useSession();

  if (status === 'detecting') {
    return (
      <div className="app viz-root">
        <Card>Загружаем…</Card>
      </div>
    );
  }

  if (mode === 'server' && !leader) {
    return <LoginScreen />;
  }

  return (
    <StoreProvider>
      <ProfileProvider>
        <Shell />
      </ProfileProvider>
    </StoreProvider>
  );
}
