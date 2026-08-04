-- Аккаунты руководителей. Остальные таблицы (assessment, invite, response,
-- leader_state) добавляются следующей миграцией вместе со слоем доступа.
--
-- Пароля нет намеренно: вход по одноразовой ссылке на почту.
CREATE TABLE leader (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL,
    name       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Почта нормализуется к нижнему регистру на уровне приложения; индекс
-- уникальный, чтобы повторный вход не плодил аккаунты.
CREATE UNIQUE INDEX leader_email_idx ON leader (email);
