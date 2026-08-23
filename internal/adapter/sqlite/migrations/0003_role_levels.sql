-- +goose Up
-- Jerarquia de roles: solo se gestiona a quien esta estrictamente por debajo.
--
-- Sin alguien por encima de todos, dos administradores en desacuerdo dejarian
-- el panel bloqueado: ninguno podria desactivar al otro. De ahi el superusuario,
-- que ademas es unico.

ALTER TABLE roles ADD COLUMN level INTEGER NOT NULL DEFAULT 0;

UPDATE roles SET level = 50 WHERE code = 'admin';
UPDATE roles SET level = 20 WHERE code = 'operator';
UPDATE roles SET level = 10 WHERE code = 'viewer';

-- Los roles creados a mano antes de esta migracion quedan por debajo de
-- operador: es el sitio mas seguro, porque no otorga poder que nadie decidio.
UPDATE roles SET level = 15 WHERE level = 0;

-- El rol se crea aqui porque la promocion de mas abajo lo necesita EN ESTE
-- MOMENTO. A partir de aqui manda EnsureRootRole (internal/app/roles.go), que
-- ademas le recalcula los permisos con el catalogo completo en cada arranque.
-- Ambos son idempotentes, asi que conviven sin pisarse.
INSERT INTO roles (code, name, is_system, level, created_at)
SELECT 'superuser', 'Superusuario', 1, 100, datetime('now')
WHERE NOT EXISTS (SELECT 1 FROM roles WHERE code = 'superuser');

-- El superusuario recibe el catalogo completo. La aplicacion lo vuelve a
-- calcular en cada arranque, asi que aqui basta con dejarlo coherente.
INSERT INTO role_permissions (role_id, permission)
SELECT (SELECT id FROM roles WHERE code = 'superuser'), permission
FROM role_permissions
WHERE role_id = (SELECT id FROM roles WHERE code = 'admin');

-- La cuenta mas antigua con rol admin pasa a superusuario: es la que creo el
-- arranque inicial, y dejar el panel sin superusuario lo bloquearia.
UPDATE users
SET role_id = (SELECT id FROM roles WHERE code = 'superuser')
WHERE id = (
    SELECT u.id FROM users u
    JOIN roles r ON r.id = u.role_id
    WHERE r.code = 'admin'
    ORDER BY u.created_at ASC, u.id ASC
    LIMIT 1
);

-- +goose Down
UPDATE users
SET role_id = (SELECT id FROM roles WHERE code = 'admin')
WHERE role_id = (SELECT id FROM roles WHERE code = 'superuser');

DELETE FROM role_permissions
WHERE role_id = (SELECT id FROM roles WHERE code = 'superuser');

DELETE FROM roles WHERE code = 'superuser';

ALTER TABLE roles DROP COLUMN level;
