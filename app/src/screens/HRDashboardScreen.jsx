import { COMPETENCIES, CLUSTER_COLOR_VAR, DESTRUCTORS, LEADERS, pseudo } from '../data';
import { Card } from '../components/ui.jsx';
import Heatmap from '../components/charts/Heatmap.jsx';

// Примечание: этот экран — независимый орг-уровневый мок (несколько руководителей).
// В реальном продукте каждая строка LEADERS — это отдельный профиль computeProfile()
// по своим собственным ответам 360°, агрегированный на бэкенде.
export default function HRDashboardScreen() {
  return (
    <section>
      <h1 className="scr-title">HR-дашборд команды</h1>
      <p className="scr-sub">
        Агрегированная карта деструкторов (риск-приоритизация) и суперсил (8 руководителей) — для орг.
        решений, а не для рейтингов.
      </p>

      <Card>
        <h3>Тепловая карта деструкторов</h3>
        <Heatmap
          leaders={LEADERS}
          columns={DESTRUCTORS}
          getValue={(li, ci) => pseudo(LEADERS[li].seed, ci + 1)}
        />
        <div className="legend" style={{ marginTop: 12 }}>
          <div className="item"><span className="swatch" style={{ background: '#cde2fb' }} /> Низкий риск</div>
          <div className="item"><span className="swatch" style={{ background: '#104281' }} /> Высокий перцентиль (норма)</div>
          <div className="item">
            <span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--status-critical)', display: 'inline-block' }} />
            {' '}Критично (&lt; 10 перц.)
          </div>
        </div>
      </Card>

      <Card>
        <h3>Карта суперсил команды</h3>
        <div className="cluster-legend">
          {Object.entries(CLUSTER_COLOR_VAR).map(([name, varname]) => (
            <div className="item" key={name}>
              <span className="dot" style={{ background: `var(${varname})` }} />
              {name}
            </div>
          ))}
        </div>
        {LEADERS.map((l) => {
          const scored = COMPETENCIES
            .map((c, ci) => ({ ...c, p: pseudo(l.seed + 50, ci + 1) }))
            .sort((a, b) => b.p - a.p)
            .slice(0, 2);
          const hasCrit = DESTRUCTORS.some((d, di) => pseudo(l.seed, di + 1) < 10);
          return (
            <div className="leader-row" key={l.name}>
              <div className="lname">{l.name}</div>
              <div className="chipset">
                {scored.map((s) => (
                  <span className="badgechip" key={s.code}>
                    <span className="dot" style={{ background: `var(${CLUSTER_COLOR_VAR[s.cluster]})` }} />
                    {s.name} · {s.p}
                  </span>
                ))}
              </div>
              {hasCrit && <span className="flag">⚠ есть критическая зона</span>}
            </div>
          );
        })}
      </Card>
    </section>
  );
}
