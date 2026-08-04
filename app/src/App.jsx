import { StoreProvider, useStore } from './state/store.jsx';
import { SessionProvider, useSession } from './state/session.jsx';
import TopNav from './components/TopNav.jsx';
import { Card } from './components/ui.jsx';
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

function Shell() {
  const { state } = useStore();
  const Screen = SCREENS[state.screen] ?? Survey360Screen;
  return (
    <div className="app viz-root">
      <TopNav />
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
      <Shell />
    </StoreProvider>
  );
}
