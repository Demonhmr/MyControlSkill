import { useState } from 'react';

const W = 640, H = 220, PAD_L = 34, PAD_R = 10, PAD_T = 14, PAD_B = 26;
const PLOT_W = W - PAD_L - PAD_R, PLOT_H = H - PAD_T - PAD_B;

export default function LineChart({ labels, series }) {
  const [hover, setHover] = useState(null);

  const x = (i) => PAD_L + (i / (labels.length - 1)) * PLOT_W;
  const y = (v) => PAD_T + PLOT_H - (v / 100) * PLOT_H;

  return (
    <div className="linechart-wrap">
      <svg className="linechart" viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none">
        {[10, 70, 90].map((v) => (
          <g key={v}>
            <line x1={PAD_L} y1={y(v)} x2={W - PAD_R} y2={y(v)} stroke="var(--gridline)" strokeWidth="1" />
            <text x={PAD_L - 6} y={y(v) + 3} textAnchor="end" fontSize="9" fill="var(--text-muted)">{v}</text>
          </g>
        ))}
        <line x1={PAD_L} y1={PAD_T + PLOT_H} x2={W - PAD_R} y2={PAD_T + PLOT_H} stroke="var(--baseline)" strokeWidth="1" />
        {labels.map((l, i) => (
          <text key={l} x={x(i)} y={H - 6} textAnchor="middle" fontSize="9.5" fill="var(--text-muted)">{l}</text>
        ))}

        {series.map((s) => {
          const pts = s.data.map((v, i) => [x(i), y(v)]);
          const d = pts.map((p, i) => `${i === 0 ? 'M' : 'L'}${p[0].toFixed(1)},${p[1].toFixed(1)}`).join(' ');
          return (
            <g key={s.name}>
              <path d={d} fill="none" stroke={s.color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
              {pts.map((p, i) => (
                <circle
                  key={i}
                  cx={p[0]} cy={p[1]} r="5"
                  fill={s.color} stroke="var(--surface-1)" strokeWidth="2"
                  style={{ cursor: 'pointer' }}
                  onMouseEnter={(e) => {
                    const r = e.target.closest('svg').getBoundingClientRect();
                    const sx = r.width / W, sy = r.height / H;
                    setHover({ x: r.left + p[0] * sx, y: r.top + p[1] * sy, label: labels[i], value: s.data[i], seriesName: s.name });
                  }}
                  onMouseLeave={() => setHover(null)}
                />
              ))}
            </g>
          );
        })}
      </svg>

      {hover && (
        <div className="floating-tooltip" style={{ position: 'fixed', left: hover.x, top: hover.y, transform: 'translate(-50%,-125%)' }}>
          <b>{hover.value}</b> · {hover.seriesName}<br />{hover.label}
        </div>
      )}
    </div>
  );
}
