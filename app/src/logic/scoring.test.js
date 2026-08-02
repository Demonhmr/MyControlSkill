import { describe, it, expect } from 'vitest';
import { computeProfile } from './scoring';

function respondent(role, competencyScores = {}, destructorScores = {}) {
  return { role, tenure: '3to12', competencyScores, destructorScores, open1: '', open2: '' };
}

describe('computeProfile', () => {
  it('не публикует живой перцентиль ниже порога респондентов', () => {
    const responses = [respondent('peer', { COM: [5, 5] })];
    const profile = computeProfile(responses);
    expect(profile.ready).toBe(false);
    const com = profile.competencies.find((c) => c.code === 'COM');
    expect(com.isLive).toBe(false);
  });

  it('считает перцентиль после порога и не смешивает в него самооценку', () => {
    const responses = [
      respondent('peer', { COM: [5, 5] }),
      respondent('subordinate', { COM: [5, 4] }),
      respondent('manager', { COM: [4, 5] }),
      respondent('self', { COM: [2, 2] }),
    ];
    const profile = computeProfile(responses);
    expect(profile.respondentCount).toBe(3);
    const com = profile.competencies.find((c) => c.code === 'COM');
    expect(com.isLive).toBe(true);
    expect(com.percentile).toBeGreaterThan(50);
  });

  it('находит слепую зону при большом расхождении self vs 360', () => {
    const responses = [
      respondent('peer', { EXP: [5, 5] }),
      respondent('subordinate', { EXP: [5, 5] }),
      respondent('manager', { EXP: [5, 5] }),
      respondent('self', { EXP: [1, 1] }),
    ];
    const profile = computeProfile(responses);
    const spot = profile.blindSpots.find((b) => b.code === 'EXP');
    expect(spot).toBeTruthy();
    expect(Math.abs(spot.delta)).toBeGreaterThanOrEqual(20);
  });
});
