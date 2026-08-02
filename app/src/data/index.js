export const COMPETENCIES = [
  { code: 'INT', name: 'Цельность и честность', cluster: 'Характер', percentile: 62 },
  { code: 'EXP', name: 'Профессиональная экспертиза', cluster: 'Личная компетентность', percentile: 91 },
  { code: 'PSA', name: 'Решение проблем и анализ', cluster: 'Личная компетентность', percentile: 68 },
  { code: 'INN', name: 'Инновации', cluster: 'Личная компетентность', percentile: 55 },
  { code: 'LRN', name: 'Скорость обучения', cluster: 'Личная компетентность', percentile: 60 },
  { code: 'RES', name: 'Ориентация на результат', cluster: 'Фокус на результате', percentile: 74 },
  { code: 'GOL', name: 'Напряжённые цели', cluster: 'Фокус на результате', percentile: 58 },
  { code: 'INI', name: 'Инициатива', cluster: 'Фокус на результате', percentile: 65 },
  { code: 'DEC', name: 'Принятие решений', cluster: 'Фокус на результате', percentile: 71 },
  { code: 'RSK', name: 'Принятие риска', cluster: 'Фокус на результате', percentile: 49 },
  { code: 'COM', name: 'Коммуникация', cluster: 'Межличностные навыки', percentile: 93 },
  { code: 'INS', name: 'Вдохновение и мотивация', cluster: 'Межличностные навыки', percentile: 78 },
  { code: 'REL', name: 'Построение отношений', cluster: 'Межличностные навыки', percentile: 88 },
  { code: 'DEV', name: 'Развитие других', cluster: 'Межличностные навыки', percentile: 52 },
  { code: 'COL', name: 'Командная работа', cluster: 'Межличностные навыки', percentile: 66 },
  { code: 'DIV', name: 'Ценность разнообразия', cluster: 'Межличностные навыки', percentile: 57 },
  { code: 'VIS', name: 'Стратегическое видение', cluster: 'Управление изменениями', percentile: 44 },
  { code: 'CHG', name: 'Поддержка изменений', cluster: 'Управление изменениями', percentile: 61 },
  { code: 'CUS', name: 'Фокус на клиенте', cluster: 'Управление изменениями', percentile: 69 },
];

export const CLUSTER_COLOR_VAR = {
  'Характер': '--cl-1',
  'Личная компетентность': '--cl-2',
  'Фокус на результате': '--cl-3',
  'Межличностные навыки': '--cl-4',
  'Управление изменениями': '--cl-5',
};

export const DESTRUCTORS = [
  { id: 'd3', name: 'Нет ясного видения и стратегии', percentile: 8, quote: '«Непонятно, куда мы вообще движемся».' },
  { id: 'd9', name: 'Сопротивляется новым идеям', percentile: 33, quote: null },
  { id: 'd2', name: 'Мирится с посредственной работой', percentile: 38, quote: null },
  { id: 'd1', name: 'Не вдохновляет окружающих', percentile: 42, quote: null },
  { id: 'd7', name: 'Не развивается, не учится на ошибках', percentile: 48, quote: null },
  { id: 'd4', name: 'Решения воспринимаются как неверные', percentile: 55, quote: null },
  { id: 'd10', name: 'Фокус на себе, а не на команде', percentile: 58, quote: null },
  { id: 'd5', name: 'Плохо взаимодействует с командой', percentile: 61, quote: null },
  { id: 'd8', name: 'Недостаточные межличностные навыки', percentile: 65, quote: null },
  { id: 'd6', name: 'Слова расходятся с делом', percentile: 70, quote: null },
];

export const COMPANIONS = {
  INT: ['DEC', 'COM', 'VIS', 'DEV', 'COL', 'RES', 'CHG'],
  EXP: ['REL', 'COM', 'DEV', 'COL', 'INN', 'VIS', 'INI'],
  PSA: ['DEC', 'INN', 'RES', 'COM', 'VIS', 'RSK', 'COL'],
  INN: ['RSK', 'CHG', 'VIS', 'PSA', 'COM', 'INS', 'COL'],
  LRN: ['INN', 'DEV', 'COL', 'COM', 'CHG', 'PSA', 'INT'],
  RES: ['DEC', 'INI', 'COM', 'INS', 'COL', 'VIS', 'DEV'],
  GOL: ['VIS', 'INS', 'COM', 'RSK', 'DEC', 'COL', 'CHG'],
  INI: ['RSK', 'DEC', 'COM', 'VIS', 'INS', 'COL', 'CHG'],
  DEC: ['INT', 'COM', 'PSA', 'RES', 'VIS', 'COL', 'RSK'],
  RSK: ['DEC', 'INN', 'VIS', 'COM', 'INT', 'COL', 'CHG'],
  COM: ['VIS', 'INS', 'INT', 'DEC', 'DEV', 'COL', 'CHG'],
  INS: ['VIS', 'COM', 'DEV', 'INT', 'GOL', 'COL', 'CHG'],
  REL: ['COM', 'COL', 'DIV', 'DEV', 'EXP', 'INT', 'INS'],
  DEV: ['COM', 'REL', 'INS', 'COL', 'DIV', 'LRN', 'INT'],
  COL: ['COM', 'DIV', 'REL', 'DEV', 'INT', 'INS', 'CHG'],
  DIV: ['COL', 'COM', 'REL', 'DEV', 'INT', 'INS', 'CUS'],
  VIS: ['COM', 'INS', 'CHG', 'RSK', 'DEC', 'GOL', 'INN'],
  CHG: ['COM', 'VIS', 'INS', 'RSK', 'COL', 'INT', 'INI'],
  CUS: ['COL', 'COM', 'INN', 'REL', 'DIV', 'VIS', 'DEC'],
};

const RATIONALE = {
  'EXP-REL': 'Экспертиза без отношений читается как «умный, но далёкий». В связке с построением отношений превращается во влияние.',
  'EXP-COM': 'Экспертное мнение долетает до команды только через ясную коммуникацию.',
  'COM-VIS': 'Коммуникация без содержания — «красиво говорит ни о чём». Видение даёт коммуникации предмет.',
  'COM-INS': 'Коммуникация в связке с вдохновением превращает информирование в мотивацию.',
  'REL-COM': 'Отношения без открытой коммуникации остаются поверхностными.',
  'REL-COL': 'Отношения усиливают командную работу — доверие один-на-один масштабируется на команду.',
  'INS-VIS': 'Вдохновение без ясной цели воспринимается как манипуляция. Видение даёт вдохновению основание.',
  'DEC-COM': 'Решения, о которых не объяснили логику, воспринимаются как произвольные.',
  'RES-DEC': 'Ориентация на результат без решительности выглядит как «требует, но не решает».',
};

export function nameOf(code) {
  return COMPETENCIES.find((c) => c.code === code)?.name ?? code;
}
export function pctOf(code) {
  return COMPETENCIES.find((c) => c.code === code)?.percentile ?? null;
}
export function rationale(target, comp) {
  return (
    RATIONALE[`${target}-${comp}`] ||
    RATIONALE[`${comp}-${target}`] ||
    `Усиливает восприятие «${nameOf(target)}» — окружение чаще видит обе компетенции в одной и той же ситуации, что закрепляет образ выдающегося уровня.`
  );
}

export const NEEDS = [
  { id: 'n1', label: 'Масштабирование команды', codes: ['DEV', 'COL', 'INS'] },
  { id: 'n2', label: 'Запуск нового продукта', codes: ['INN', 'VIS', 'RSK'] },
  { id: 'n3', label: 'Кризис доверия в команде', codes: ['INT', 'COM', 'REL'] },
  { id: 'n4', label: 'Выход на новый рынок', codes: ['CUS', 'VIS', 'RSK'] },
  { id: 'n5', label: 'Оптимизация процессов', codes: ['RES', 'PSA', 'DEC'] },
];

export const SCENARIOS = {
  EXP: { trigger: 'Команда сомневается в правильности вашего технического решения', bad: '«Я эксперт, просто доверьтесь мне»', good: 'Признать неопределённость и показать ход рассуждений', script: '«Вот как я рассуждал и какие риски видел. Что вы видите такого, чего не вижу я?»' },
  COM: { trigger: 'Нужно сообщить команде непопулярное решение', bad: 'Написать решение в чат без объяснений', good: 'Назвать решение прямо и объяснить логику до того, как появятся вопросы', script: '«Решение такое-то. Вот почему. Что для вас в этом самое сложное?»' },
  REL: { trigger: 'Конфликт между двумя ключевыми членами команды', bad: 'Игнорировать, надеясь, что само решится', good: 'Провести прямой разговор с обеими сторонами вместе', script: '«Я вижу напряжение между вами. Предлагаю проговорить это здесь и сейчас»' },
  RES: { trigger: 'Команда показала хороший, но не выдающийся результат', bad: 'Похвалить и не поднимать планку', good: 'Признать результат и сразу обозначить следующую высоту', script: '«Это хорошо. А что нужно, чтобы сделать это выдающимся?»' },
  DEC: { trigger: 'Нужно решение при неполных данных, команда ждёт', bad: 'Затягивать до идеальной уверенности', good: 'Принять решение открыто, обозначив риск и план Б', script: '«Я решаю так, зная X и не зная Y. Если ошибусь — вот план Б»' },
  INS: { trigger: 'Команда устала и демотивирована после сложного релиза', bad: '«Соберитесь, у нас дедлайны»', good: 'Признать усталость и связать усилия со смыслом', script: '«Я вижу, как вы устали. И вот почему то, что вы сделали, было важно»' },
  DESTRUCTOR_VIS: { trigger: 'Команда не понимает, куда движется компания/продукт', bad: '«Просто делайте свою работу, стратегией займусь я»', good: 'Явно сформулировать картину и связать с ней текущие задачи каждого', script: '«Вот куда мы идём в этом квартале и почему то, что вы делаете сейчас, для этого важно»' },
};
export function scenarioFor(code) {
  return (
    SCENARIOS[code] || {
      trigger: 'Ситуация несогласия, важная и эмоционально заряженная для команды',
      bad: 'Защищать решение, спорить с несогласием',
      good: 'Сначала создать безопасность для высказывания несогласия, затем говорить по сути',
      script: '«Я знаю, что вы не в восторге. Давайте честно: с чем вы не согласны?»',
    }
  );
}

export function zoneOf(p) {
  if (p >= 90) return { color: 'var(--series-blue-strong)', label: 'Суперсила' };
  if (p >= 70) return { color: 'var(--series-blue)', label: 'Кандидат' };
  return { color: 'var(--zone-muted)', label: 'Не в фокусе' };
}

export const LEADERS = [
  { name: 'А. Петрова', seed: 3 },
  { name: 'И. Смирнов', seed: 11 },
  { name: 'Е. Ковалёва', seed: 7 },
  { name: 'Д. Орлов', seed: 19 },
  { name: 'М. Захарова', seed: 5 },
  { name: 'С. Волков', seed: 23 },
  { name: 'Н. Соколова', seed: 2 },
  { name: 'Р. Гриценко', seed: 17 },
];
export function pseudo(seed, i) {
  return Math.round(Math.abs(Math.sin(seed * 13.37 + i * 7.77)) * 100);
}
export function blueForPct(p) {
  const steps = [
    [20, '#cde2fb'], [35, '#9ec5f4'], [50, '#6da7ec'], [65, '#3987e5'],
    [75, '#2a78d6'], [85, '#1c5cab'], [95, '#104281'], [101, '#0d366b'],
  ];
  for (const [max, hex] of steps) if (p <= max) return hex;
  return steps[steps.length - 1][1];
}
