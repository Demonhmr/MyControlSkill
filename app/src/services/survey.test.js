import { describe, it, expect } from 'vitest';
import { toSubmission } from './survey.js';

describe('toSubmission', () => {
  it('различает нетронутый пункт, «не могу оценить» и оценку', () => {
    const { answers } = toSubmission({
      tenure: 'gt12',
      competencyScores: { COM: [5, null], INT: [undefined, 3] },
      destructorScores: {},
    });

    // Нетронутые пункты не отправляются вовсе, «не могу оценить» — явным null.
    expect(answers).toEqual([
      { kind: 'competency', code: 'COM', itemIndex: 0, value: 5 },
      { kind: 'competency', code: 'COM', itemIndex: 1, value: null },
      { kind: 'competency', code: 'INT', itemIndex: 1, value: 3 },
    ]);
  });

  it('размечает деструкторы отдельной шкалой', () => {
    const { answers } = toSubmission({
      tenure: 'lt3',
      competencyScores: {},
      destructorScores: { d3: [1, 2] },
    });

    expect(answers).toEqual([
      { kind: 'destructor', code: 'd3', itemIndex: 0, value: 1 },
      { kind: 'destructor', code: 'd3', itemIndex: 1, value: 2 },
    ]);
  });

  it('не отправляет пустые открытые ответы', () => {
    const { openAnswers } = toSubmission({
      tenure: 'gt12',
      competencyScores: {},
      destructorScores: {},
      open1: '   ',
      open2: 'Разобрал провал спокойно.',
    });

    expect(openAnswers).toEqual([{ questionIndex: 1, text: 'Разобрал провал спокойно.' }]);
  });

  it('переносит срок наблюдения как есть', () => {
    const submission = toSubmission({ tenure: '3to12', competencyScores: {}, destructorScores: {} });
    expect(submission.tenure).toBe('3to12');
    expect(submission.answers).toEqual([]);
    expect(submission.openAnswers).toEqual([]);
  });
});
