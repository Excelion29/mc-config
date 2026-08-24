-- +goose Up
-- Ajustes con los que nace y se mantiene un mundo.
--
-- Se guardan en columnas y no en un JSON suelto porque son pocos, fijos y se
-- consultan: "damelos mundos en creativo" es una pregunta razonable, y contra
-- un JSON no se responde con un indice.
--
-- Van separados en dos grupos por como se comportan:
--
--   generacion  seed, level_type, structures, bonus_chest
--               Dan forma al terreno UNA vez. Cambiarlos despues no reescribe
--               lo ya generado, asi que la interfaz solo los ofrece al crear.
--
--   reglas      gamemode, difficulty, allow_commands, pvp, max_players
--               Se releen en cada arranque. El panel las reescribe siempre,
--               igual que hace con la lista de permitidos: si alguien cambia
--               la dificultad dentro del juego, vale para esa sesion y al
--               reiniciar vuelve a mandar el panel.

-- Generacion
ALTER TABLE worlds ADD COLUMN structures  INTEGER NOT NULL DEFAULT 1;
ALTER TABLE worlds ADD COLUMN bonus_chest INTEGER NOT NULL DEFAULT 0;

-- Reglas
ALTER TABLE worlds ADD COLUMN gamemode       TEXT    NOT NULL DEFAULT 'survival';
ALTER TABLE worlds ADD COLUMN difficulty     TEXT    NOT NULL DEFAULT 'normal';
ALTER TABLE worlds ADD COLUMN allow_commands INTEGER NOT NULL DEFAULT 0;
ALTER TABLE worlds ADD COLUMN pvp            INTEGER NOT NULL DEFAULT 1;
ALTER TABLE worlds ADD COLUMN max_players    INTEGER NOT NULL DEFAULT 12;

-- Los mundos que ya existian se quedan con estos valores por defecto, que son
-- los que el panel venia escribiendo: survival, easy y 12 jugadores. Se pone
-- 'easy' donde ya habia mundos para no cambiarles la dificultad por sorpresa.
UPDATE worlds SET difficulty = 'easy' WHERE origin = 'imported';

-- +goose Down
ALTER TABLE worlds DROP COLUMN max_players;
ALTER TABLE worlds DROP COLUMN pvp;
ALTER TABLE worlds DROP COLUMN allow_commands;
ALTER TABLE worlds DROP COLUMN difficulty;
ALTER TABLE worlds DROP COLUMN gamemode;
ALTER TABLE worlds DROP COLUMN bonus_chest;
ALTER TABLE worlds DROP COLUMN structures;
