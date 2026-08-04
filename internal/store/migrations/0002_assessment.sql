-- Раунд оценки 360°. Раундов у руководителя несколько: повторные замеры —
-- это то, на чём строится пульс-трекер.
CREATE TABLE assessment (
    id         TEXT PRIMARY KEY,
    leader_id  TEXT NOT NULL REFERENCES leader (id) ON DELETE CASCADE,
    title      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    closed_at  TEXT
);

CREATE INDEX assessment_leader_idx ON assessment (leader_id, created_at);

-- Приглашение респонденту: одна ссылка — один заполняющий.
--
-- Хранится хэш токена, а не сам токен: из дампа базы нельзя восстановить
-- рабочие ссылки. Почта нужна только чтобы отправить приглашение и не
-- пригласить одного человека дважды.
CREATE TABLE invite (
    id            TEXT PRIMARY KEY,
    assessment_id TEXT NOT NULL REFERENCES assessment (id) ON DELETE CASCADE,
    token_hash    TEXT NOT NULL UNIQUE,
    role          TEXT NOT NULL,
    email         TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
    used_at       TEXT
);

CREATE INDEX invite_assessment_idx ON invite (assessment_id);

-- Заполненная анкета.
--
-- Ссылки на invite здесь нет намеренно. Приглашение знает почту респондента,
-- и если бы ответ на него ссылался, руководитель (или утёкший дамп) связал бы
-- оценки с конкретными людьми — анонимность 360° на этом заканчивается.
-- Одноразовость ссылки обеспечивается отметкой invite.used_at в той же
-- транзакции, что и вставка ответа, без сохранения связи между ними.
CREATE TABLE response (
    id            TEXT PRIMARY KEY,
    assessment_id TEXT NOT NULL REFERENCES assessment (id) ON DELETE CASCADE,
    role          TEXT NOT NULL,
    tenure        TEXT NOT NULL,
    submitted_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE INDEX response_assessment_idx ON response (assessment_id, role);

-- Оценка одного пункта анкеты.
--
-- Хранится по пунктам, а не средним по компетенции: нормативную базу
-- перцентилей предстоит калибровать на реальных данных, и для этого нужны
-- исходные распределения, а не уже усреднённые значения.
--
-- value = NULL — это «не могу оценить». Такой ответ отличается от
-- отсутствующей строки (пункт вообще не показывали или пропустили) и в
-- среднее не идёт.
CREATE TABLE answer (
    response_id TEXT NOT NULL REFERENCES response (id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('competency', 'destructor')),
    code        TEXT NOT NULL,
    item_index  INTEGER NOT NULL CHECK (item_index >= 0),
    value       INTEGER CHECK (value IS NULL OR value BETWEEN 1 AND 5),
    PRIMARY KEY (response_id, kind, code, item_index)
) WITHOUT ROWID;

-- Свободные ответы на открытые вопросы. Отдельной таблицей, а не колонками
-- в response: число вопросов со временем поменяется.
CREATE TABLE open_answer (
    response_id    TEXT NOT NULL REFERENCES response (id) ON DELETE CASCADE,
    question_index INTEGER NOT NULL CHECK (question_index >= 0),
    text           TEXT NOT NULL,
    PRIMARY KEY (response_id, question_index)
) WITHOUT ROWID;

-- Рабочее состояние руководителя: отметка о проработке деструктора,
-- выбранная точка роста, отмеченные интересы и потребности команды.
--
-- JSON, а не колонки: это состояние экранов, оно меняется вместе с UI и
-- запросов по отдельным полям не требует.
CREATE TABLE leader_state (
    leader_id  TEXT PRIMARY KEY REFERENCES leader (id) ON DELETE CASCADE,
    data       TEXT NOT NULL DEFAULT '{}',
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

-- Записи из тренажёра и пульс-трекера. В отличие от остального состояния
-- это временной ряд: пульс-трекер строит по нему динамику, поэтому нужна
-- выборка по дате.
CREATE TABLE reflection (
    id         TEXT PRIMARY KEY,
    leader_id  TEXT NOT NULL REFERENCES leader (id) ON DELETE CASCADE,
    code       TEXT NOT NULL,
    text       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE INDEX reflection_leader_idx ON reflection (leader_id, created_at);
