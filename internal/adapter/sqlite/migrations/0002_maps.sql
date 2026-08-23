-- +goose Up
-- Biblioteca de mapas (F2).
--
-- El archivo NO se guarda aqui: la base solo tiene los metadatos y el hash.
-- Los binarios viven en disco, indexados por sha256 (adapter/storage).

CREATE TABLE maps (
    id          INTEGER PRIMARY KEY,
    -- name es el nombre ya limpio de codigos de formato; raw_name conserva el
    -- original, porque los colores son parte de como el autor lo diseno.
    name        TEXT    NOT NULL,
    raw_name    TEXT    NOT NULL DEFAULT '',
    edition     TEXT    NOT NULL,
    version     TEXT    NOT NULL DEFAULT '',
    file_name   TEXT    NOT NULL,
    size_bytes  INTEGER NOT NULL,
    -- UNIQUE sobre el hash: subir dos veces el mismo archivo, aunque venga
    -- renombrado, se detecta como duplicado.
    sha256      TEXT    NOT NULL UNIQUE,
    has_icon    INTEGER NOT NULL DEFAULT 0,
    uploaded_by INTEGER,
    created_at  TEXT    NOT NULL,
    CONSTRAINT valid_edition CHECK (edition IN ('bedrock', 'java')),
    -- SET NULL y no CASCADE: borrar al usuario que lo subio no debe borrar el
    -- mapa ni el mundo que haya dentro.
    FOREIGN KEY (uploaded_by) REFERENCES users (id) ON DELETE SET NULL
);

CREATE INDEX idx_maps_edition ON maps (edition);
CREATE INDEX idx_maps_created ON maps (created_at);

-- +goose Down
DROP TABLE maps;
