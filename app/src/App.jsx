import { StoreProvider, useStore } from './state/store.jsx';
import TopNav from './components/TopNav.jsx';
import Survey360Screen from './screens/Survey360Screen.jsx';
import DestructorsScreen from './screens/DestructorsScreen.jsx';
import StrengthMapScreen from './screens/StrengthMapScreen.jsx';
import GrowthPointScreen from './screens/GrowthPointScreen.jsx';
import PlanScreen from './screens/PlanScreen.jsx';
import TrainerScreen from './screens/TrainerScreen.jsx';
import PulseScreen from './screens/PulseScreen.jsx';
import HRDashboardScreen from './screens/HRDashboardScreen.jsx';

const SCREENS = {
  survey: Survey360Screen,
  destructors: DestructorsScreen,
  strength: StrengthMapScreen,
  growth: GrowthPointScreen,
  plan: PlanScreen,
  trainer: TrainerScreen,
  pulse: PulseScreen,
  hr: HRDashboardScreen,
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
    <StoreProvider>
      <Shell />
    </StoreProvider>
  );
}
