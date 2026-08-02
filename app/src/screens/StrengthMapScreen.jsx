import { useStore, hasCriticalDestructor } from '../state/store.jsx';
import { useProfile } from '../state/useProfile.js';
import { Card, Banner } from '../components/ui.jsx';
import BarChart from '../components/charts/BarChart.jsx';

export default function StrengthMapScreen() {
  const { state } = useStore();
  const profile = useProfile();
  const blocked = hasCriticalDestructor(state, profile.destructors);

  return (
    <section>
      <h1 className="scr-title">Карта сильных сторон</h1>
      <p className="scr-sub">
        19 лидерских компетенций. Развитие в диапазоне 10–70 перцентиля почти не меняет восприятие
        лидерства — такие компетенции намеренно приглушены.
      </p>

      {!profile.ready && (
        <Banner title="Демонстрационные значения">
          Показаны исходные (демо) перцентили — соберите минимум 3 внешних ответа 360°, чтобы увидеть реальные данные.
        </Banner>
      )}

      {blocked && (
        <Banner title="Просмотр без активации плана">
          Пока критическая зона не проработана, карту можно смотреть, но выбор точки роста недоступен.
        </Banner>
      )}

      <Card>
        <BarChart competencies={profile.competencies} />
      </Card>

      {profile.blindSpots.length > 0 && (
        <Card>
          <h3>Слепые зоны (самооценка vs 360°)</h3>
          {profile.blindSpots.map((b) => (
            <div key={b.code} className="destructor-row">
              <span className="name">{b.name}</span>
              <span className="pct">
                вы: {b.selfPct} · окружение: {b.othersPct} ({b.delta > 0 ? 'переоценка' : 'недооценка'})
              </span>
            </div>
          ))}
        </Card>
      )}
    </section>
  );
}
