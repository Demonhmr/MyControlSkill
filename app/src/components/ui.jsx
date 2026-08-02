export function Card({ children, className = '' }) {
  return <div className={`card ${className}`}>{children}</div>;
}

export function Banner({ tone = 'critical', title, children }) {
  return (
    <div className={`banner ${tone === 'ok' ? 'ok' : ''}`}>
      <div className="ic">{tone === 'ok' ? '✓' : '!'}</div>
      <div>
        <b>{title}</b>
        <div className="banner-body">{children}</div>
      </div>
    </div>
  );
}

export function Badge({ tone = 'neutral', children }) {
  return (
    <span className={`badge ${tone}`}>
      <span className="dot" />
      {children}
    </span>
  );
}

export function Button({ variant, children, ...rest }) {
  return (
    <button className={`btn ${variant ?? ''}`} {...rest}>
      {children}
    </button>
  );
}

export function ToggleTableButton({ showingTable, onClick }) {
  return (
    <button className="toggle-table" onClick={onClick}>
      {showingTable ? 'Показать как график' : 'Показать как таблицу'}
    </button>
  );
}

export function Legend({ items }) {
  return (
    <div className="legend">
      {items.map((it) => (
        <div className="item" key={it.label}>
          <span className="swatch" style={{ background: it.color }} />
          {it.label}
        </div>
      ))}
    </div>
  );
}
