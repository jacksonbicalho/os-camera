-- gate por classificador para notificacao e exibicao no rodape
ALTER TABLE camera_state_classifiers ADD COLUMN notify_enabled INTEGER NOT NULL DEFAULT 1;
ALTER TABLE camera_state_classifiers ADD COLUMN footer_enabled INTEGER NOT NULL DEFAULT 0;

-- KV generico de permissoes/preferencias por usuario (reutilizavel)
CREATE TABLE user_permissions (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key     TEXT    NOT NULL,
    value   TEXT    NOT NULL,
    PRIMARY KEY (user_id, key)
);

-- backfill: classificadores existentes notificam todos os usuarios atuais (snapshot)
INSERT OR IGNORE INTO user_permissions (user_id, key, value)
SELECT u.id, 'state_notify:' || c.id, '1'
FROM users u CROSS JOIN camera_state_classifiers c;
