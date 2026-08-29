-- +goose Up
-- Ajustes globales del panel.
--
-- Clave y valor, sin columnas por ajuste: son pocos, se leen de uno en uno y
-- ninguno se consulta ni se filtra. Una tabla por ajuste seria una migracion
-- cada vez que aparezca uno nuevo.
--
-- El primero que vive aqui es el MODO DE AUTENTICACION, y es global a
-- proposito (D-17): con modo sin conexion el UUID de un jugador se calcula de
-- su nombre, y con modo normal lo asigna Mojang. Si cada mundo eligiera su
-- modo, la misma persona tendria DOS identidades segun donde juegue, y la
-- lista de permitidos tendria que llevar las dos y saber cual toca. Una sola
-- identidad por persona vale mas que la flexibilidad de mezclar.

CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Se arranca en modo normal, que es el seguro: Mojang autentica y nadie puede
-- usar el nombre de otro. Pasar a sin conexion es una decision explicita y
-- exige que AuthMe y FastLogin esten instalados (D-07).
INSERT INTO settings (key, value, updated_at)
VALUES ('auth_mode', 'online', datetime('now'));

-- +goose Down
DROP TABLE settings;
