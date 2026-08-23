-- +goose Up
-- Identidad real de Xbox Live y rastro de la primera conexion.
--
-- permissions.json de Bedrock identifica a los operadores por XUID y no por
-- gamertag. El XUID ya existe -lo tiene la persona desde que creo su cuenta-
-- pero el panel no lo conoce hasta que se conecta y el servidor lo escribe en
-- su log.
--
-- De ahi el alta en dos fases, que es lo que hace segura toda la idea:
--
--   1. Se anade a la allow-list por gamertag  -> ya puede entrar
--   2. Entra por primera vez                  -> aprendemos su XUID
--   3. Solo entonces se le puede dar operador
--
-- Como nadie que no este en la allow-list llega a conectarse, solo se capturan
-- XUIDs de gente previamente aprobada.

ALTER TABLE players ADD COLUMN xuid TEXT NOT NULL DEFAULT '';

-- Cuando se le vio por primera vez. NULL = todavia no ha entrado nunca.
ALTER TABLE players ADD COLUMN first_seen TEXT;

-- Buscar por XUID no hace falta hoy, pero si buscar por gamertag al leer el
-- log: cada conexion pregunta "quien es este nombre". Ya hay un UNIQUE sobre
-- gamertag que sirve de indice, asi que no se anade otro.

-- +goose Down
ALTER TABLE players DROP COLUMN first_seen;
ALTER TABLE players DROP COLUMN xuid;
