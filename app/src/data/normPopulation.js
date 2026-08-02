function mulberry32(seed) {
  return function () {
    seed |= 0; seed = (seed + 0x6D2B79F5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
function codeSeed(code) {
  let h = 0;
  for (let i = 0; i < code.length; i++) h = (h * 31 + code.charCodeAt(i)) >>> 0;
  return h;
}

function buildPopulation(code, size = 300) {
  const rand = mulberry32(codeSeed(code));
  const pop = [];
  for (let i = 0; i < size; i++) {
    const u1 = rand() || 1e-6, u2 = rand();
    const z = Math.sqrt(-2 * Math.log(u1)) * Math.cos(2 * Math.PI * u2);
    pop.push(Math.max(1, Math.min(5, 3.2 + z * 0.55)));
  }
  return pop.sort((a, b) => a - b);
}

const cache = {};
export function percentileRank(value, code) {
  if (!cache[code]) cache[code] = buildPopulation(code);
  const pop = cache[code];
  let lo = 0, hi = pop.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (pop[mid] <= value) lo = mid + 1; else hi = mid;
  }
  return Math.round((lo / pop.length) * 100);
}
