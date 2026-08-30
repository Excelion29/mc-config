-- +goose Up
-- Los paquetes de texturas pasan a ser RECURSOS.
--
-- Un recurso es algo de fuera que un mundo necesita y que el panel no aloja.
-- Hoy solo hay un tipo, los paquetes de texturas, pero el tipo va en la tabla
-- desde ya: los que vengan -mods, mapas de referencia- tienen que caber como una
-- fila mas, no como una tabla mas (D-18).
--
-- Va en una migracion NUEVA y no reescribiendo la 0016 porque aquella ya se
-- habia ejecutado. Goose se guia por el numero, asi que cambiar el archivo no lo
-- vuelve a ejecutar: la base se queda con el esquema viejo y el codigo pidiendo
-- el nuevo, y el sintoma es un "no such column" en una pantalla que no tiene
-- nada que ver.
--
-- Se copian los datos en vez de empezar de cero: puede haber paquetes ya
-- guardados, y perderlos por un cambio de nombre seria gratuito.

CREATE TABLE resources (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT    NOT NULL DEFAULT 'texture_pack',

    -- Lo unico obligatorio. Es lo que identifica al recurso de verdad.
    url  TEXT    NOT NULL,

    -- El nombre es una MASCARA del enlace, no un requisito: si esta, se ensena
    -- en vez de la URL y al pulsarlo se va al enlace. Vacio es normal.
    name TEXT    NOT NULL DEFAULT '',
    -- auto_name es el titulo que el panel saco de la propia pagina al anadirlo.
    -- Aparte del nombre puesto a mano, para no confundir lo que alguien decidio
    -- con lo que el panel adivino.
    auto_name TEXT NOT NULL DEFAULT '',

    -- Huella con la que el cliente reconoce el archivo que ya tiene. Vacia si no
    -- se pudo calcular: entonces se lo descarga en cada conexion.
    sha1 TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',

    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TEXT    NOT NULL
);

INSERT INTO resources (id, kind, url, name, auto_name, sha1, note, created_by, created_at)
SELECT id, 'texture_pack', url, name, '', sha1, note, created_by, created_at FROM packs;

-- El enlace es la identidad: dos filas con el mismo serian dos entradas que hay
-- que corregir por separado el dia que el archivo se mueva.
CREATE UNIQUE INDEX idx_resources_url ON resources(url);

CREATE TABLE world_resources (
    world_id    INTEGER NOT NULL REFERENCES worlds(id)    ON DELETE CASCADE,
    resource_id INTEGER NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    -- El principal es el que el servidor aplica solo. server.properties tiene
    -- UNA clave "resource-pack", asi que mas de uno no lo puede cumplir nadie.
    principal   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (world_id, resource_id)
);

INSERT INTO world_resources (world_id, resource_id, principal)
SELECT world_id, pack_id, activo FROM world_packs;

-- Un solo principal por mundo, garantizado por la base y no solo por el codigo.
-- El indice parcial solo cuenta las filas principales: varios secundarios por
-- mundo son lo normal.
CREATE UNIQUE INDEX idx_world_resource_principal
    ON world_resources(world_id) WHERE principal = 1;

DROP INDEX idx_world_pack_activo;
DROP TABLE world_packs;
DROP INDEX idx_packs_url;
DROP TABLE packs;

-- Si el recurso principal se exige o se ofrece. Es del mundo y no del recurso:
-- el mismo paquete es imprescindible en un mapa de aventura y prescindible en
-- cualquier otro sitio.
ALTER TABLE worlds ADD COLUMN resource_required INTEGER NOT NULL DEFAULT 0;
UPDATE worlds SET resource_required = pack_required;
ALTER TABLE worlds DROP COLUMN pack_required;

-- +goose Down
ALTER TABLE worlds ADD COLUMN pack_required INTEGER NOT NULL DEFAULT 0;
UPDATE worlds SET pack_required = resource_required;
ALTER TABLE worlds DROP COLUMN resource_required;

CREATE TABLE packs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    url        TEXT    NOT NULL,
    sha1       TEXT    NOT NULL DEFAULT '',
    note       TEXT    NOT NULL DEFAULT '',
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TEXT    NOT NULL
);
INSERT INTO packs (id, name, url, sha1, note, created_by, created_at)
SELECT id, name, url, sha1, note, created_by, created_at FROM resources;
CREATE UNIQUE INDEX idx_packs_url ON packs(url);

CREATE TABLE world_packs (
    world_id INTEGER NOT NULL REFERENCES worlds(id) ON DELETE CASCADE,
    pack_id  INTEGER NOT NULL REFERENCES packs(id)  ON DELETE CASCADE,
    activo   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (world_id, pack_id)
);
INSERT INTO world_packs (world_id, pack_id, activo)
SELECT world_id, resource_id, principal FROM world_resources;
CREATE UNIQUE INDEX idx_world_pack_activo ON world_packs(world_id) WHERE activo = 1;

DROP INDEX idx_world_resource_principal;
DROP TABLE world_resources;
DROP INDEX idx_resources_url;
DROP TABLE resources;
