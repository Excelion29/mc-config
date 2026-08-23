-- +goose Up
-- Lista maestra de jugadores (F4, D-13).
--
-- La verdad vive AQUI. Los allowlist.json de cada instancia son derivados: se
-- regeneran desde esta tabla. Si alguien los edita a mano en disco, el panel
-- los sobrescribe, y es intencional.

CREATE TABLE players (
    id         INTEGER PRIMARY KEY,
    -- Se guarda tal cual se escribio: Bedrock compara el gamertag EXACTO,
    -- mayusculas incluidas. Por eso el UNIQUE tambien distingue mayusculas.
    gamertag   TEXT    NOT NULL UNIQUE,
    -- Para el hito 2: en Java la identidad es otra (usuario de AuthMe), asi
    -- que una misma persona necesita dos identificadores.
    java_name  TEXT    NOT NULL DEFAULT '',
    note       TEXT    NOT NULL DEFAULT '',
    -- Operador DENTRO del juego, no en el panel. Son permisos distintos.
    is_op      INTEGER NOT NULL DEFAULT 0,
    active     INTEGER NOT NULL DEFAULT 1,
    created_at TEXT    NOT NULL
);

CREATE INDEX idx_players_active ON players (active);

-- +goose Down
DROP TABLE players;
