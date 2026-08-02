import { useState, useRef } from 'react';
import { zoneOf } from '../../data';
import { ToggleTableButton, Legend } from '../ui.jsx';

export default function BarChart({ competencies }) {
  const [showTable, setShowTable] = useState(false);
  const [tip, setTip] = useState(null);
  const trackRefs = useRef({});

  const sorted = [...competencies].sort((a, b) => b.percentile - a.percentile);

  const onEnter = (c, el) => {
    const r = el.getBoundingClientRect();
    const z = zoneOf(c.percentile);
    setTip({
      x: r.left + (r.width * c.percentile) / 100,
      y: r.top,
      text: `${c.name} — ${c.percentile}-й перцентиль (${z.label})${c.isLive ? '' : ' · демо'}`,
    });
  };

  return (
    <>
      <Legend
        items={[
          { color: 'var(--zone-muted)', label: 'Не в фокусе (10–70 перц.)' },
          { color: 'var(--series-blue)', label: 'Кандидат в точку роста (70–90)' },
          { color: 'var(--series-blue-strong)', label: '★ Суперсила (90+)' },
        ]}
      />

      {!showTable && (
        <div>
          {sorted.map((c) => {
            const z = zoneOf(c.percentile);
            return (
              <div className="bar-row" key={c.code}>
                <div className="lbl">
                  {c.name}
                  <span className="cl">{c.cluster}</span>
                </div>
                <div
                  className="bar-track"
                  tabIndex={0}
                  ref={(el) => (trackRefs.current[c.code] = el)}
                  onMouseEnter={() => onEnter(c, trackRefs.current[c.code])}
                  onMouseLeave={() => setTip(null)}
                >
                  <div className="refline" style={{ left: '70%' }} />
                  <div className="refline" style={{ left: '90%' }} />
                  <div className="bar-fill" style={{ width: `${c.percentile}%`, background: z.color }} />
                </div>
                <span className="bar-val">
                  {c.percentile}
                  {c.percentile >= 90 && <span className="bar-star">★</span>}
                </span>
              </div>
            );
          })}
          <div className="footnote" style={{ marginLeft: 240 }}>
            Тонкие вертикальные линии — пороги 70 и 90 перцентиля
          </div>
        </div>
      )}

      {showTable && (
        <table className="twin">
          <thead>
            <tr><th>Компетенция</th><th>Кластер</th><th>Перцентиль</th><th>Зона</th></tr>
          </thead>
          <tbody>
            {sorted.map((c) => {
              const z = zoneOf(c.percentile);
              return (
                <tr key={c.code}>
                  <td>{c.name}</td><td>{c.cluster}</td>
                  <td className="num">{c.percentile}</td><td>{z.label}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}

      <ToggleTableButton showingTable={showTable} onClick={() => setShowTable((v) => !v)} />

      {tip && (
        <div className="floating-tooltip" style={{ left: tip.x, top: tip.y, transform: 'translate(-50%,-130%)' }}>
          {tip.text}
        </div>
      )}
    </>
  );
}
