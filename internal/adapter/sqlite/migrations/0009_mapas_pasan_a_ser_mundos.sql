-- +goose Up
-- "Mapa" pasa a llamarse "mundo".
--
-- El nombre viejo se quedo corto al planificar Java. Un mapa es UNA FORMA de
-- conseguir un mundo -importar un archivo-, pero en Java tiene sentido crear
-- uno desde cero con una semilla, sin archivo de partida. Llamar "mapa" a eso
-- obliga a explicarlo cada vez.
--
-- Lo que el servidor carga es un mundo. De donde salio es otra cosa, y por eso
-- se anade la columna "origin": hoy todos vienen de un archivo importado, pero
-- la que viene despues no.

ALTER TABLE maps RENAME TO worlds;

-- De donde salio este mundo. Hoy solo hay un valor; en el hito 2 habra
-- 'created' para los que nacen de una semilla.
ALTER TABLE worlds ADD COLUMN origin TEXT NOT NULL DEFAULT 'imported';

-- La instancia apunta a un mundo, no a un mapa.
ALTER TABLE instances RENAME COLUMN map_id TO world_id;

-- Los indices siguen a la tabla al renombrarla, pero sus nombres se quedan
-- con el prefijo viejo y eso confunde al leer un EXPLAIN. Se rehacen.
DROP INDEX IF EXISTS idx_maps_edition;
DROP INDEX IF EXISTS idx_maps_created;
CREATE INDEX idx_worlds_edition ON worlds (edition);
CREATE INDEX idx_worlds_created ON worlds (created_at);

DROP INDEX IF EXISTS idx_instances_map;
CREATE INDEX idx_instances_world ON instances (world_id);

-- +goose Down
DROP INDEX IF EXISTS idx_instances_world;
CREATE INDEX idx_instances_map ON instances (map_id);
DROP INDEX IF EXISTS idx_worlds_created;
DROP INDEX IF EXISTS idx_worlds_edition;
CREATE INDEX idx_maps_created ON maps (created_at);
CREATE INDEX idx_maps_edition ON maps (edition);
ALTER TABLE instances RENAME COLUMN world_id TO map_id;
ALTER TABLE worlds DROP COLUMN origin;
ALTER TABLE worlds RENAME TO maps;
