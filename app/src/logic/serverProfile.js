import { COMPETENCIES, DESTRUCTORS, nameOf } from '../data';

/**
 * Приводит серверный профиль к форме, которую рисуют экраны.
 *
 * Сервер отдаёт только коды и числа: названия компетенций, кластеры и цитаты
 * живут на клиенте, дублировать их на сервере незачем. Здесь эти два слоя
 * склеиваются.
 *
 * Демонстрационные перцентили из data/index.js при этом затираются: у них та
 * же форма, что у настоящих, и оставить их означало бы подмешать витрину
 * прототипа в результат реального замера. Если сервер по коду ничего не
 * прислал, значение остаётся пустым.
 */
export function adaptServerProfile(server) {
  const competencyByCode = new Map((server.competencies ?? []).map((s) => [s.code, s]));
  const destructorByCode = new Map((server.destructors ?? []).map((s) => [s.code, s]));

  const competencies = COMPETENCIES.map((c) => merge(c, competencyByCode.get(c.code)));
  const destructors = DESTRUCTORS.map((d) => merge(d, destructorByCode.get(d.id)));

  const blindSpots = (server.blindSpots ?? []).map((b) => ({
    code: b.code,
    name: nameOf(b.code),
    selfPct: b.selfPercentile,
    othersPct: b.othersPercentile,
    delta: b.delta,
  }));

  return {
    competencies,
    destructors,
    respondentCount: server.respondentCount ?? 0,
    ready: Boolean(server.ready),
    blindSpots,
    // Самооценка приходит внутрь расчёта на сервере и наружу не выдаётся:
    // по ней вместе с остальными анкетами восстанавливались бы ответы.
    selfResponse: null,
  };
}

function merge(meta, score) {
  return {
    ...meta,
    percentile: score?.percentile ?? null,
    raw: score?.raw ?? null,
    isLive: score?.percentile != null,
  };
}
