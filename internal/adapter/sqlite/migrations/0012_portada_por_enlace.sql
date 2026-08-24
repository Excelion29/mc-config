-- +goose Up
-- Portada de un mundo creado, como ENLACE y no como archivo.
--
-- Un mapa importado trae su portada dentro del .mcworld y se guarda con el.
-- Un mundo creado no tiene ninguna, y pedirle a alguien que suba una imagen
-- para algo que ni siquiera existe todavia en disco es pedir de mas.
--
-- Se guarda la URL y no la imagen, por lo mismo que los paquetes de texturas:
-- sin almacenamiento y sin crecimiento de disco, que en esta VPS ya nos
-- preocupa (M-2).
--
-- Lo que se acepta a cambio: la imagen es de otro. Si la mueven o la borran,
-- aparece rota y desde el panel no se puede arreglar.

ALTER TABLE worlds ADD COLUMN icon_url TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE worlds DROP COLUMN icon_url;
