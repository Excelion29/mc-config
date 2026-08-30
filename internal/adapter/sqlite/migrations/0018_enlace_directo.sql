-- +goose Up
-- Si el enlace devuelve el ARCHIVO o una pagina.
--
-- Antes se adivinaba por la extension de la URL, y se equivocaba en los dos
-- sentidos: un CDN puede servir el paquete desde "/pack?id=123" -sin extension y
-- perfectamente valido- y una pagina puede llamarse "/algo.zip" sin serlo.
--
-- Se sabe de verdad porque el panel ya abre el enlace una vez para calcular la
-- huella: mirar de paso que tipo de contenido devuelve no cuesta nada.
--
-- Van DOS columnas y no una: "es una pagina" y "no se pudo comprobar" no son lo
-- mismo. Con una sola, un fallo de red pasaria por pagina y el recurso quedaria
-- marcado como manual para siempre sin que nadie lo revisara.
ALTER TABLE resources ADD COLUMN directo  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE resources ADD COLUMN probado  INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE resources DROP COLUMN probado;
ALTER TABLE resources DROP COLUMN directo;
