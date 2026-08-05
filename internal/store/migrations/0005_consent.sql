-- Согласие руководителя на показ его профиля HR-службе организации.
--
-- Без согласия эйчар видит человека в составе, но не видит его чисел. По
-- умолчанию согласия нет: молчание согласием не считается.
ALTER TABLE org_member ADD COLUMN profile_consent_at TEXT;

-- Журнал выдачи и отзыва.
--
-- Текущее состояние есть в org_member и читается дёшево, но одного его мало:
-- спор о том, давал ли человек согласие и когда именно, разрешается только
-- записью. Строки отсюда не удаляются и не правятся.
CREATE TABLE consent_event (
    id         TEXT PRIMARY KEY,
    leader_id  TEXT NOT NULL REFERENCES leader (id) ON DELETE CASCADE,
    org_id     TEXT NOT NULL REFERENCES org (id) ON DELETE CASCADE,
    -- 1 — согласие выдано, 0 — отозвано.
    granted    INTEGER NOT NULL CHECK (granted IN (0, 1)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX consent_event_leader_idx ON consent_event (leader_id, created_at);
