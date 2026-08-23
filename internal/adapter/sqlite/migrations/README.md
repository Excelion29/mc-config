# Migraciones

Historial de todo lo que le ha pasado al esquema, en orden. Se conserva
completo a propósito: es el registro de cómo llegó la base a ser lo que es.

## Índice

| # | Archivo | Qué hizo |
|---|---|---|
| 0001 | `init.sql` | Esquema inicial: `roles`, `role_permissions`, `users`, `sessions`, `audit_log` |
| 0002 | `maps.sql` | Biblioteca de mapas: tabla `maps`, indexada por `sha256` del archivo |
| 0003 | `role_levels.sql` | Jerarquía: columna `level`, rol `superuser`, y promoción del admin más antiguo |
| 0004 | `only_root_is_system.sql` | Sólo el superusuario es rol de sistema; los demás pasan a ser editables |

## Cómo crear una

```bash
go run ./cmd/mcvps -new-migration "actualizar mapas"
#   Creada  internal/adapter/sqlite/migrations/0005_actualizar_mapas.sql

go run ./cmd/mcvps -migrations   # ver el estado
```

La numeración es secuencial y la pone el comando. Las migraciones pendientes se
aplican solas al arrancar el panel.

## Reglas

**Sólo se añade.** Una migración ya aplicada en cualquier sitio no se edita ni
se borra nunca. Si la `0007` estaba mal, no se corrige: se escribe la `0008` que
arregla lo que hizo la `0007`. Editarla dejaría a las bases que ya la aplicaron
apuntando a algo que no existe.

**Sólo esquema y datos existentes.** Lo que necesite lógica de la aplicación
—hashear una contraseña, leer el catálogo de permisos— va en el arranque, no
aquí. Ver `EnsureSuperuser` en `internal/app/auth.go`.

**SQL portable** (D-14): sin funciones específicas de SQLite, para que migrar a
PostgreSQL sea escribir un adaptador y no reescribir el historial. Las fechas
las escribe Go en RFC3339; la base no genera ninguna.

**Escribe siempre el `Down`.** Y si algo no se puede deshacer, déjalo escrito y
explica por qué.

## Reiniciar la base

`goose` lleva su contabilidad en `goose_db_version`. **Vaciar esa tabla no
reinicia nada**: las tablas siguen existiendo pero el registro dice que no, y el
arranque falla con `no next version found`.

Para reiniciar de verdad: borrar el archivo `.db`, o eliminar **todas** las
tablas incluida la de goose.

> ⚠️ Al borrar datos desde DBeaver, ejecuta antes `PRAGMA foreign_keys = ON;`.
> SQLite las trae desactivadas por defecto: la aplicación las activa al
> conectarse, pero DBeaver no, así que los `CASCADE` no se aplican y quedan
> filas huérfanas.

## Nota sobre 0003

Esa migración crea el rol `superuser` por SQL, y `EnsureRootRole` también lo
crea al arrancar. No es un descuido: la migración necesitaba que el rol
existiera **en ese momento** para poder promover al administrador más antiguo
que ya estaba en la base.

De ahí en adelante manda el arranque, que además le recalcula los permisos con
el catálogo completo. Ambos son idempotentes, así que conviven sin pisarse.
