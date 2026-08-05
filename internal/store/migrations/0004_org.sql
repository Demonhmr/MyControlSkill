-- Организация: несколько руководителей и те, кто смотрит по ним сводку.
CREATE TABLE org (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- Участие в организации.
--
-- Роль hr даёт доступ к сводке по всем участникам, leader — только к своим
-- раундам. Роль складывается с личным кабинетом, а не заменяет его: у
-- эйчара тоже есть свои раунды 360°.
--
-- leader_id уникален: одна организация на человека. Для пилота этого
-- достаточно, а совмещение сразу упёрлось бы в вопрос, чью сводку показывать.
CREATE TABLE org_member (
    org_id    TEXT NOT NULL REFERENCES org (id) ON DELETE CASCADE,
    leader_id TEXT NOT NULL REFERENCES leader (id) ON DELETE CASCADE,
    role      TEXT NOT NULL CHECK (role IN ('hr', 'leader')),
    joined_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (org_id, leader_id)
);

CREATE UNIQUE INDEX org_member_leader_idx ON org_member (leader_id);
