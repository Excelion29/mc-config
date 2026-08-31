-- +goose Up
-- La version de un complemento, cuando no es la de fabrica.
--
-- La version por defecto sigue fijada en el codigo, y por el mismo motivo de
-- siempre: un plugin de autenticacion que se actualiza solo es lo ultimo que
-- queremos. Lo que cambia es que ahora se puede subir SIN desplegar.
--
-- Solo hay fila para lo que alguien decidio cambiar. Sin fila, manda el codigo:
-- asi un panel recien instalado arranca con lo verificado a mano, y lo que se
-- guarda aqui es siempre una decision explicita de una persona.
--
-- Se guarda tambien el nombre del archivo. Se deduce de la URL, pero hace falta
-- tenerlo aparte para poder BORRAR el .jar viejo al cambiar de version: dos
-- versiones del mismo plugin en la misma carpeta las carga las dos.
CREATE TABLE plugin_versions (
    plugin_id  TEXT    PRIMARY KEY,
    url        TEXT    NOT NULL,
    file       TEXT    NOT NULL,
    updated_by INTEGER NOT NULL REFERENCES users(id),
    updated_at TEXT    NOT NULL
);

-- +goose Down
DROP TABLE plugin_versions;
