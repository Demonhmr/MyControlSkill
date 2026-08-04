-- Одноразовая ссылка для входа.
--
-- Пароля у руководителя нет: он получает ссылку на почту и по ней входит.
-- Хранится хэш токена — из дампа базы рабочие ссылки не восстановить.
--
-- Почта лежит здесь, а не берётся из leader: аккаунт может ещё не
-- существовать, первый переход по ссылке его и создаёт.
CREATE TABLE login_token (
    id         TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    email      TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    expires_at TEXT NOT NULL,
    used_at    TEXT
);

-- Протухшие токены чистятся пачкой, поэтому индекс по сроку.
CREATE INDEX login_token_expires_idx ON login_token (expires_at);

-- Сессия руководителя. Идентификатор сессии живёт в cookie, в базе — его хэш.
CREATE TABLE session (
    id         TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    leader_id  TEXT NOT NULL REFERENCES leader (id) ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    expires_at TEXT NOT NULL
);

CREATE INDEX session_leader_idx ON session (leader_id);
CREATE INDEX session_expires_idx ON session (expires_at);
