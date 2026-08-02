export default function RatingScale({ value, onChange }) {
  return (
    <div style={{ display: 'flex', gap: 6, marginTop: 4, flexWrap: 'wrap' }}>
      {[1, 2, 3, 4, 5].map((n) => (
        <button
          key={n}
          type="button"
          onClick={() => onChange(n)}
          className={`chip ${value === n ? 'selected' : ''}`}
          style={{ minWidth: 32, textAlign: 'center' }}
        >
          {n}
        </button>
      ))}
      <button
        type="button"
        onClick={() => onChange(null)}
        className={`chip ${value === null ? 'selected' : ''}`}
      >
        Не могу оценить
      </button>
    </div>
  );
}
