-- +goose NO TRANSACTION
-- +goose Up
-- Un mundo puede NACER VACIO, sin importar ningun archivo.
--
-- Es lo normal cuando quieres jugar: generas un mundo y entras. Importar un
-- mapa es el caso especial, no al reves. Y no es cosa de Java: Bedrock tambien
-- lo hace -se vio sin querer en F3, cuando un bind mount mal puesto acabo con
-- un "CREATING VANILLA WORLD"-.
--
-- Esto obliga a tocar sha256, que era NOT NULL UNIQUE. Un mundo creado no
-- tiene archivo y por tanto no tiene hash. Con cadena vacia solo cabria UNO en
-- toda la base, porque el UNIQUE los considerarnia duplicados; con NULL caben
-- todos, porque SQLite trata cada NULL como distinto.
--
-- SQLite no permite quitar un NOT NULL con ALTER, asi que hay que reconstruir
-- la tabla. Es el procedimiento estandar y va sin transaccion porque
-- foreign_keys no se puede cambiar dentro de una.

PRAGMA foreign_keys = OFF;

-- +goose StatementBegin
CREATE TABLE worlds_nueva (
    id          INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL,
    raw_name    TEXT    NOT NULL DEFAULT '',
    edition     TEXT    NOT NULL,
    version     TEXT    NOT NULL DEFAULT '',

    -- De donde salio el mundo. Es lo que decide si los campos de archivo
    -- significan algo.
    origin      TEXT    NOT NULL DEFAULT 'imported',

    -- Campos del archivo importado. Vacios o NULL en un mundo creado.
    file_name   TEXT    NOT NULL DEFAULT '',
    size_bytes  INTEGER NOT NULL DEFAULT 0,
    -- NULL en los creados. Sigue siendo UNIQUE para los importados: subir dos
    -- veces el mismo archivo, aunque venga renombrado, se detecta.
    sha256      TEXT    UNIQUE,
    has_icon    INTEGER NOT NULL DEFAULT 0,

    -- Campos del mundo creado. Vacios en un mundo importado.
    --
    -- La semilla se guarda como TEXTO y no como numero a proposito: Minecraft
    -- acepta palabras -"hola" es una semilla valida- y las convierte el mismo.
    -- Guardarla como entero perderia justo las que la gente escribe a mano.
    seed        TEXT    NOT NULL DEFAULT '',
    -- normal, flat, largeBiomes, amplified... El valor exacto lo valida cada
    -- adaptador, porque no coinciden entre ediciones.
    level_type  TEXT    NOT NULL DEFAULT '',

    uploaded_by INTEGER,
    created_at  TEXT    NOT NULL,

    CONSTRAINT valid_edition CHECK (edition IN ('bedrock', 'java')),
    CONSTRAINT valid_origin  CHECK (origin IN ('imported', 'created')),
    FOREIGN KEY (uploaded_by) REFERENCES users (id) ON DELETE SET NULL
);
-- +goose StatementEnd

INSERT INTO worlds_nueva (id, name, raw_name, edition, version, origin, file_name, size_bytes, sha256, has_icon, uploaded_by, created_at)
SELECT id, name, raw_name, edition, version, origin, file_name, size_bytes, sha256, has_icon, uploaded_by, created_at FROM worlds;

DROP TABLE worlds;
ALTER TABLE worlds_nueva RENAME TO worlds;

CREATE INDEX idx_worlds_edition ON worlds (edition);
CREATE INDEX idx_worlds_created ON worlds (created_at);
CREATE INDEX idx_worlds_origin  ON worlds (origin);

PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

-- +goose StatementBegin
CREATE TABLE worlds_vieja (
    id          INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL,
    raw_name    TEXT    NOT NULL DEFAULT '',
    edition     TEXT    NOT NULL,
    version     TEXT    NOT NULL DEFAULT '',
    file_name   TEXT    NOT NULL,
    size_bytes  INTEGER NOT NULL,
    sha256      TEXT    NOT NULL UNIQUE,
    has_icon    INTEGER NOT NULL DEFAULT 0,
    uploaded_by INTEGER,
    created_at  TEXT    NOT NULL,
    origin      TEXT    NOT NULL DEFAULT 'imported',
    CONSTRAINT valid_edition CHECK (edition IN ('bedrock', 'java')),
    FOREIGN KEY (uploaded_by) REFERENCES users (id) ON DELETE SET NULL
);
-- +goose StatementEnd

-- Los mundos creados se pierden al revertir: no tienen archivo y la tabla
-- vieja no admite sha256 nulo.
INSERT INTO worlds_vieja (id, name, raw_name, edition, version, file_name, size_bytes, sha256, has_icon, uploaded_by, created_at, origin)
SELECT id, name, raw_name, edition, version, file_name, size_bytes, sha256, has_icon, uploaded_by, created_at, origin
FROM worlds WHERE sha256 IS NOT NULL;

DROP TABLE worlds;
ALTER TABLE worlds_vieja RENAME TO worlds;

CREATE INDEX idx_worlds_edition ON worlds (edition);
CREATE INDEX idx_worlds_created ON worlds (created_at);

PRAGMA foreign_keys = ON;
