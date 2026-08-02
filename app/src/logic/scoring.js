import { COMPETENCIES, DESTRUCTORS } from '../data';
import { percentileRank } from '../data/normPopulation';

export const MIN_RESPONDENTS = 3;

function avgFor(responses, mapKey, code) {
  const vals = [];
  responses.forEach((r) => {
    const answers = r[mapKey]?.[code];
    if (!answers) return;
    answers.forEach((v) => { if (v != null) vals.push(v); });
  });
  return vals.length ? vals.reduce((a, b) => a + b, 0) / vals.length : null;
}

export function computeProfile(responses) {
  const external = responses.filter((r) => r.role !== 'self');
  const ready = external.length >= MIN_RESPONDENTS;

  const competencies = COMPETENCIES.map((c) => {
    const raw = ready ? avgFor(external, 'competencyScores', c.code) : null;
    const percentile = raw != null ? percentileRank(raw, c.code) : c.percentile;
    return { ...c, percentile, raw, isLive: raw != null };
  });

  const destructors = DESTRUCTORS.map((d) => {
    const raw = ready ? avgFor(external, 'destructorScores', d.id) : null;
    const percentile = raw != null ? percentileRank(raw, d.id) : d.percentile;
    return { ...d, percentile, raw, isLive: raw != null };
  });

  const selfResponse = responses.find((r) => r.role === 'self') || null;
  const blindSpots = selfResponse
    ? competencies
        .map((c) => {
          if (!c.isLive) return null;
          const selfVals = (selfResponse.competencyScores?.[c.code] || []).filter((v) => v != null);
          if (!selfVals.length) return null;
          const selfAvg = selfVals.reduce((a, b) => a + b, 0) / selfVals.length;
          const selfPct = percentileRank(selfAvg, c.code);
          const delta = selfPct - c.percentile;
          return Math.abs(delta) >= 20 ? { code: c.code, name: c.name, selfPct, othersPct: c.percentile, delta } : null;
        })
        .filter(Boolean)
    : [];

  return { competencies, destructors, respondentCount: external.length, ready, blindSpots, selfResponse };
}
