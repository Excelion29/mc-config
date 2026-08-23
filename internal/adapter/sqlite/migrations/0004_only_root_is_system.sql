-- +goose Up
-- El unico rol que el sistema impone es el superusuario.
--
-- Administrador, Operador y Espectador se crearon en migraciones anteriores
-- marcados como "de sistema", asi que no se podian borrar ni editar del todo.
-- Pasan a ser roles normales: quedan como punto de partida util, pero se
-- gestionan como cualquier otro, que es la idea de que los roles sean datos.

UPDATE roles SET is_system = 0 WHERE code <> 'superuser';

-- +goose Down
UPDATE roles SET is_system = 1 WHERE code IN ('admin', 'operator', 'viewer');
