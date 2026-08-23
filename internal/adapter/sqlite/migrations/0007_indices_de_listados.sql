-- +goose Up
-- Indices para los listados paginados y el filtro del registro.
--
-- Al anadir la busqueda por accion, la consulta pasaba a recorrer el registro
-- ENTERO en cada carga. Con treinta filas no se nota; el registro es la tabla
-- que mas crece del panel -una fila por cada cosa que hace cualquiera- y es
-- precisamente donde una tabla completa deja de caber.
--
-- Es deuda que nacio con la funcionalidad, no antes: el filtro es de este mes.

CREATE INDEX idx_audit_action ON audit_log (action);

-- La lista de jugadores se ordena por "activos primero, luego alfabetico", y
-- se pagina. Un indice solo sobre active no cubre el segundo criterio, asi que
-- SQLite tenia que ordenar el resultado en memoria antes de recortarlo.
DROP INDEX IF EXISTS idx_players_active;
CREATE INDEX idx_players_orden ON players (active DESC, gamertag ASC);

-- Los usuarios se listan por antiguedad y tampoco tenian indice para ese
-- orden. Son pocos hoy, pero la consulta es la misma con diez que con mil.
CREATE INDEX idx_users_created ON users (created_at, id);

-- +goose Down
DROP INDEX IF EXISTS idx_users_created;
DROP INDEX IF EXISTS idx_players_orden;
CREATE INDEX idx_players_active ON players (active);
DROP INDEX IF EXISTS idx_audit_action;
