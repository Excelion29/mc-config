-- +goose Up
-- Esquema inicial de MCVPS (F1), con RBAC.
-- SQL portable a proposito (D-14): sin funciones especificas de SQLite.
-- Las fechas las escribe Go en RFC3339, no la base de datos.

-- Los ROLES son datos: se crean y se editan desde el panel.
-- Los PERMISOS no tienen tabla propia: su catalogo vive en el codigo
-- (domain/permission.go), porque un permiso solo existe si algun handler lo
-- comprueba. Aqui solo se guarda que permisos tiene cada rol.
CREATE TABLE roles (
    id         INTEGER PRIMARY KEY,
    code       TEXT    NOT NULL UNIQUE,
    name       TEXT    NOT NULL,
    is_system  INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL
);

CREATE TABLE role_permissions (
    role_id    INTEGER NOT NULL,
    permission TEXT    NOT NULL,
    PRIMARY KEY (role_id, permission),
    FOREIGN KEY (role_id) REFERENCES roles (id) ON DELETE CASCADE
);

CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    email         TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    role_id       INTEGER NOT NULL,
    active        INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT    NOT NULL,
    -- RESTRICT y no CASCADE: borrar un rol nunca debe llevarse por delante a
    -- las personas que lo tenian. El panel obliga a reasignarlas antes.
    FOREIGN KEY (role_id) REFERENCES roles (id) ON DELETE RESTRICT
);

CREATE INDEX idx_users_role ON users (role_id);

CREATE TABLE sessions (
    token      TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL,
    created_at TEXT    NOT NULL,
    expires_at TEXT    NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE INDEX idx_sessions_user    ON sessions (user_id);
CREATE INDEX idx_sessions_expires ON sessions (expires_at);

-- Registro de acciones (requisito de D-08).
-- user_email se guarda desnormalizado a proposito: si un usuario se borra, el
-- registro de lo que hizo debe sobrevivir. Un log que se borra con el usuario
-- no sirve para auditar.
CREATE TABLE audit_log (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER,
    user_email TEXT    NOT NULL,
    action     TEXT    NOT NULL,
    detail     TEXT    NOT NULL DEFAULT '',
    ip         TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL
);

CREATE INDEX idx_audit_created ON audit_log (created_at);
CREATE INDEX idx_audit_user    ON audit_log (user_id);

-- +goose Down
DROP TABLE audit_log;
DROP TABLE sessions;
DROP TABLE users;
DROP TABLE role_permissions;
DROP TABLE roles;
