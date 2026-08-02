import { describe, it, expect } from 'vitest';
import { hasCriticalDestructor } from './store.jsx';

describe('hasCriticalDestructor', () => {
  const destructors = [{ id: 'd3', percentile: 8 }, { id: 'd1', percentile: 50 }];

  it('блокирует, если есть деструктор ниже 10 перцентиля и он не подтверждён', () => {
    expect(hasCriticalDestructor({ destructorAcknowledged: false }, destructors)).toBe(true);
  });

  it('снимает блокировку после подтверждения проработки', () => {
    expect(hasCriticalDestructor({ destructorAcknowledged: true }, destructors)).toBe(false);
  });

  it('не блокирует, если критических зон нет', () => {
    expect(hasCriticalDestructor({ destructorAcknowledged: false }, [{ id: 'd1', percentile: 50 }])).toBe(false);
  });
});
