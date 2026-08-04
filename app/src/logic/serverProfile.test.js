import { describe, it, expect } from 'vitest';
import { adaptServerProfile } from './serverProfile.js';
import { COMPETENCIES, DESTRUCTORS } from '../data';

const SERVER = {
  ready: true,
  respondentCount: 3,
  competencies: [{ code: 'COM', raw: 4.5, percentile: 88 }],
  destructors: [{ code: 'd3', raw: 1.5, percentile: 7 }],
  blindSpots: [{ code: 'EXP', selfPercentile: 20, othersPercentile: 90, delta: -70 }],
};

describe('adaptServerProfile', () => {
  it('склеивает числа сервера с названиями и кластерами клиента', () => {
    const profile = adaptServerProfile(SERVER);

    const com = profile.competencies.find((c) => c.code === 'COM');
    expect(com.percentile).toBe(88);
    expect(com.raw).toBe(4.5);
    expect(com.isLive).toBe(true);
    expect(com.name).toBe(COMPETENCIES.find((c) => c.code === 'COM').name);
    expect(com.cluster).toBe(COMPETENCIES.find((c) => c.code === 'COM').cluster);

    const d3 = profile.destructors.find((d) => d.id === 'd3');
    expect(d3.percentile).toBe(7);
    expect(d3.name).toBe(DESTRUCTORS.find((d) => d.id === 'd3').name);
  });

  // У демонстрационных перцентилей та же форма, что у настоящих: оставить их
  // означало бы показать витрину прототипа как результат замера команды.
  it('затирает демонстрационные перцентили там, где сервер молчит', () => {
    const profile = adaptServerProfile(SERVER);

    const untouched = profile.competencies.find((c) => c.code === 'INT');
    expect(untouched.percentile).toBeNull();
    expect(untouched.isLive).toBe(false);
    expect(untouched.name).toBe(COMPETENCIES.find((c) => c.code === 'INT').name);

    const demoValue = COMPETENCIES.find((c) => c.code === 'INT').percentile;
    expect(demoValue).toEqual(expect.any(Number));
    expect(untouched.percentile).not.toBe(demoValue);
  });

  it('переводит слепые зоны в форму экранов', () => {
    const profile = adaptServerProfile(SERVER);

    expect(profile.blindSpots).toEqual([
      {
        code: 'EXP',
        name: COMPETENCIES.find((c) => c.code === 'EXP').name,
        selfPct: 20,
        othersPct: 90,
        delta: -70,
      },
    ]);
  });

  it('не отдаёт наружу самооценку', () => {
    // По самооценке вместе с остальными анкетами восстанавливались бы ответы,
    // поэтому её на клиенте нет вовсе.
    expect(adaptServerProfile(SERVER).selfResponse).toBeNull();
  });

  it('переживает пустой ответ', () => {
    const profile = adaptServerProfile({});

    expect(profile.ready).toBe(false);
    expect(profile.respondentCount).toBe(0);
    expect(profile.competencies).toHaveLength(COMPETENCIES.length);
    expect(profile.competencies.every((c) => c.percentile === null)).toBe(true);
    expect(profile.blindSpots).toEqual([]);
  });
});
