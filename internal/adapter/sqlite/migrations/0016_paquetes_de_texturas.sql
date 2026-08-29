-- +goose Up
-- Biblioteca de paquetes de texturas.
--
-- Tabla propia y no columnas en worlds: el mismo paquete vale para varios
-- mapas, y un mapa puede tener varios. Con columnas habria que repetir el
-- enlace en cada mundo y corregirlos de uno en uno el dia que el autor lo mueva.
--
-- Del archivo no se guarda nada, solo el enlace: Java sirve los paquetes por URL
-- y el cliente los descarga solo al conectarse.
CREATE TABLE packs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    url        TEXT    NOT NULL,
    -- Hash con el que el cliente reconoce el paquete que ya tiene. Vacio si no
    -- se pudo calcular: entonces se lo descarga en cada conexion, que es peor
    -- pero no impide nada.
    sha1       TEXT    NOT NULL DEFAULT '',
    note       TEXT    NOT NULL DEFAULT '',
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TEXT    NOT NULL
);

CREATE UNIQUE INDEX idx_packs_url ON packs(url);

-- Que paquetes lleva cada mundo.
CREATE TABLE world_packs (
    world_id INTEGER NOT NULL REFERENCES worlds(id) ON DELETE CASCADE,
    pack_id  INTEGER NOT NULL REFERENCES packs(id)  ON DELETE CASCADE,
    -- El unico que el servidor aplica solo. server.properties tiene UNA clave
    -- "resource-pack", asi que mas de uno activo no lo puede cumplir nadie.
    activo   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (world_id, pack_id)
);

-- Un solo activo por mundo, garantizado por la base y no solo por el codigo.
-- El indice parcial solo cuenta las filas activas, que es justo lo que hace
-- falta: varios inactivos por mundo son normales.
CREATE UNIQUE INDEX idx_world_pack_activo ON world_packs(world_id) WHERE activo = 1;

-- Si el paquete activo se exige o se ofrece. Es del mundo y no del paquete: el
-- mismo paquete puede ser imprescindible en un mapa de aventura y opcional en
-- otro sitio.
ALTER TABLE worlds ADD COLUMN pack_required INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE worlds DROP COLUMN pack_required;
DROP TABLE world_packs;
DROP TABLE packs;
