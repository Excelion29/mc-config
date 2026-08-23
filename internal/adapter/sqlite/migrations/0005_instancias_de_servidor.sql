-- +goose Up
-- Instancias de servidor (F3): una version, un mapa y su configuracion.
--
-- Por D-02 solo hay una encendida a la vez, asi que no hace falta repartir
-- puertos: todas usan el estandar de su edicion.

CREATE TABLE instances (
    id           INTEGER PRIMARY KEY,
    name         TEXT    NOT NULL,
    -- slug se usa como nombre de contenedor y de carpeta: minusculas, numeros
    -- y guiones. UNIQUE porque Docker no admite dos contenedores con el mismo
    -- nombre y porque dos instancias no pueden compartir carpeta.
    slug         TEXT    NOT NULL UNIQUE,
    edition      TEXT    NOT NULL,
    version      TEXT    NOT NULL DEFAULT '',
    map_id       INTEGER,
    -- level_name es el nombre de la carpeta dentro de worlds/, que es lo que
    -- server.properties apunta con level-name.
    level_name   TEXT    NOT NULL,
    container_id TEXT    NOT NULL DEFAULT '',
    port         INTEGER NOT NULL,
    state        TEXT    NOT NULL DEFAULT 'stopped',
    memory_mb    INTEGER NOT NULL,
    cpus         REAL    NOT NULL,
    created_at   TEXT    NOT NULL,
    last_started TEXT,
    CONSTRAINT valid_edition CHECK (edition IN ('bedrock', 'java')),
    CONSTRAINT valid_state CHECK (state IN ('stopped','starting','running','stopping','failed')),
    -- SET NULL y no CASCADE: borrar un mapa de la biblioteca no debe llevarse
    -- por delante el mundo ya instalado ni lo jugado en el.
    FOREIGN KEY (map_id) REFERENCES maps (id) ON DELETE SET NULL
);

CREATE INDEX idx_instances_state ON instances (state);
CREATE INDEX idx_instances_map   ON instances (map_id);

-- +goose Down
DROP TABLE instances;
