-- +goose Up
-- Huella de la definicion con la que se creo el contenedor.
--
-- Existe para detectar que lo que el panel quiere ya no es lo que hay
-- levantado. El caso que lo destapo: el modo de autenticacion viaja como
-- variable de entorno, y esas se fijan al CREAR el contenedor. Reiniciar no las
-- cambia, asi que abrir el acceso a cuentas no premium no surtia efecto nunca
-- en un servidor ya creado, y no habia forma de notarlo: arrancaba bien.
--
-- Vacia en las instancias que ya existen, a proposito: no coincide con ninguna
-- huella, asi que el primer arranque despues de esto rehace el contenedor. Es
-- justo lo que hace falta para que los servidores viejos recojan el modo.
ALTER TABLE instances ADD COLUMN spec_hash TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE instances DROP COLUMN spec_hash;
