import { useState } from 'react';
import { useStore } from '../state/store.jsx';
import { useProfile } from '../state/useProfile.js';
import { Card, ToggleTableButton, Legend } from '../components/ui.jsx';
import LineChart from '../components/charts/LineChart.jsx';

export default function PulseScreen() {
  const { state } = useStore();
  const profile = useProfile();
  const [showTable, setShowTable] = useState(false);

  if (!state.growthPoint && !state.destructorAcknowledged) {
    return (
      <section>
        <h1 className="scr-title">Пульс-трекер</h1>
        <Card>
          <div className="muted">
            Пульс-трекер станет активным после выбора точки роста (и/или подтверждения работы над критической зоной).
          </div>
        </Card>
      </section>
    );
  }

  const base = state.growthPoint ? profile.competencies.find((c) => c.code === state.growthPoint) : null;
  const growthSeries = base ? [base.percentile, base.percentile + 5, base.percentile + 9, Math.min(97, base.percentile + 13)] : null;
  const destr = profile.destructors.find((d) => d.id === 'd3');
  const destrSeries = state.destructorAcknowledged && destr ? [destr.percentile, 15, 24, 34] : null;
  const labels = ['Baseline', '+6 нед.', '+12 нед.', '+18 нед.'];

  const series = [
    growthSeries && { name: base.name, color: 'var(--series-blue)', data: growthSeries },
    destrSeries && { name: 'Деструктор: нет видения', color: 'var(--series-red)', data: destrSeries },
  ].filter(Boolean);

  return (
    <section>
      <h1 className="scr-title">Пульс-трекер</h1>
      <p className="scr-sub">
        Короткая точечная переоценка выбранной связки (не весь профиль) раз в 6–8 недель — отслеживаем сдвиг перцентиля.
      </p>

      <Card>
        <h3>Динамика перцентиля</h3>

        {!showTable && series.length > 0 && (
          <>
            <LineChart labels={labels} series={series} />
            <Legend items={series.map((s) => ({ color: s.color, label: s.name }))} />
          </>
        )}

        {showTable && (
          <table className="twin">
            <thead>
              <tr>
                <th>Точка</th>
                {series.map((s) => <th key={s.name}>{s.name}</th>)}
              </tr>
            </thead>
            <tbody>
              {labels.map((l, i) => (
                <tr key={l}>
                  <td>{l}</td>
                  {series.map((s) => <td className="num" key={s.name}>{s.data[i]}</td>)}
                </tr>
              ))}
            </tbody>
          </table>
        )}

        {series.length > 0 && <ToggleTableButton showingTable={showTable} onClick={() => setShowTable((v) => !v)} />}
      </Card>
    </section>
  );
}
