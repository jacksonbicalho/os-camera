-- State classification: por câmera, um ou mais classificadores que olham um
-- recorte fixo (crop, normalizado 0–1) e dizem um estado (aberto/fechado, etc.).
-- Config apenas (a inferência/escrita do estado é feita pelo agendador, S3).
CREATE TABLE IF NOT EXISTS camera_state_classifiers (
    id                       INTEGER PRIMARY KEY AUTOINCREMENT,
    camera_id                TEXT    NOT NULL REFERENCES cameras(id) ON DELETE CASCADE,
    name                     TEXT    NOT NULL,
    model                    TEXT    NOT NULL DEFAULT 'custom-cls',
    threshold                REAL    NOT NULL DEFAULT 0.8,
    trigger_motion           INTEGER NOT NULL DEFAULT 1,
    trigger_interval_seconds INTEGER NOT NULL DEFAULT 0,
    crop_x                   REAL    NOT NULL,
    crop_y                   REAL    NOT NULL,
    crop_w                   REAL    NOT NULL,
    crop_h                   REAL    NOT NULL,
    min_consecutive          INTEGER NOT NULL DEFAULT 3,
    enabled                  INTEGER NOT NULL DEFAULT 1,
    created_at               DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Classes possíveis de cada classificador (≥2, validado na aplicação).
CREATE TABLE IF NOT EXISTS camera_state_classes (
    classifier_id INTEGER NOT NULL REFERENCES camera_state_classifiers(id) ON DELETE CASCADE,
    label         TEXT    NOT NULL,
    display_order INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (classifier_id, label)
);

-- Histórico de transições de estado (o corrente é o registro mais recente).
CREATE TABLE IF NOT EXISTS camera_state_history (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    classifier_id INTEGER NOT NULL REFERENCES camera_state_classifiers(id) ON DELETE CASCADE,
    state         TEXT    NOT NULL,
    confidence    REAL    NOT NULL DEFAULT 0,
    changed_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_state_classifiers_camera ON camera_state_classifiers(camera_id);
CREATE INDEX IF NOT EXISTS idx_state_history_classifier ON camera_state_history(classifier_id, changed_at);
