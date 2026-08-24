-- +goose Up
-- El UUID de Java de un jugador.
--
-- whitelist.json y ops.json de Java identifican por UUID, no por nombre. A
-- diferencia de Bedrock -donde el XUID no se conoce hasta que la persona entra
-- (H-J-8)- en Java el UUID se puede preguntar a Mojang a partir del nombre, asi
-- que se puede dar acceso a alguien que nunca ha jugado.
--
-- Es una TERCERA identidad, distinta del gamertag de Bedrock y de la cuenta del
-- panel (H-J-9). La misma persona tiene nombres distintos en cada una.

ALTER TABLE players ADD COLUMN java_uuid TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE players DROP COLUMN java_uuid;
