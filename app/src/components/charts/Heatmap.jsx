import { useState } from 'react';
import { blueForPct } from '../../data';

export default function Heatmap({ leaders, columns, getValue }) {
  const [showTable, setShowTable] = useState(false);
  const [tip, setTip] = useState(null);

  const cells = [];
  leaders.forEach((l, li) => columns.forEach((c, ci) => {
    const p = getValue(li, ci);
    cells.push({ leader: l.name, col: c.name, p, crit: p < 10 });
  }));

  return (
    <>
      {!showTable && (
        <div className="heatmap-scroll">
          <table className="heat">
            <thead>
              <tr>
                <th></th>
                {columns.map((c) => <th className="col" key={c.id ?? c.name}>{c.name}</th>)}
              </tr>
            </thead>
            <tbody>
              {leaders.map((l, li) => (
                <tr key={l.name}>
                  <th>{l.name}</th>
                  {columns.map((c, ci) => {
                    const p = getValue(li, ci);
                    const crit = p < 10;
                    return (
                      <td
                        key={c.id ?? c.name}
                        className="cell"
                        style={{ background: blueForPct(p) }}
                        onMouseEnter={(e) => {
                          const r = e.target.getBoundingClientRect();
                          setTip({ x: r.left + r.width / 2, y: r.top - 8, leader: l.name, col: c.name, p, crit });
                        }}
                        onMouseLeave={() => setTip(null)}
                      >
                        {crit && <span className="rd" />}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showTable && (
        <>
          <table className="twin">
            <thead><tr><th>Руководитель</th><th>Деструктор</th><th>Перцентиль</th></tr></thead>
            <tbody>
              {cells.filter((c) => c.crit).map((c, i) => (
                <tr key={i}><td>{c.leader}</td><td>{c.col}</td><td className="num">{c.p}</td></tr>
              ))}
              {!cells.some((c) => c.crit) && (
                <tr><td colSpan={3} className="muted">Критических зон не обнаружено</td></tr>
              )}
            </tbody>
          </table>
          <div className="footnote">В таблице — только критические ячейки (&lt; 10 перц.). Полная матрица — на графике.</div>
        </>
      )}

      <button className="toggle-table" onClick={() => setShowTable((v) => !v)}>
        {showTable ? 'Показать как график' : 'Показать как таблицу'}
      </button>

      {tip && (
        <div className="floating-tooltip" style={{ position: 'fixed', left: tip.x, top: tip.y, transform: 'translate(-50%,-100%)' }}>
          <b>{tip.p}</b> перц. · {tip.leader}<br />{tip.col}
          {tip.crit && <><br /><span style={{ color: '#ff9b9b' }}>Критическая зона</span></>}
        </div>
      )}
    </>
  );
}
