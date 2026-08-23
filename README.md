# MCVPS

Panel web para gestionar servidores de Minecraft en una VPS: importar un mapa,
prepararle un servidor con su versión, y arrancarlo o pararlo desde el navegador
sin tocar la consola.

Escrito en Go, sin frontend aparte: un único binario que sirve la interfaz y
habla con Docker.

## Estado

| Fase | Qué hace | |
|---|---|---|
| F1 | Autenticación, RBAC con jerarquía, registro de acciones | ✅ |
| F2 | Biblioteca de mapas: subida, validación y detección de edición/versión | ✅ |
| F3 | Instancias de servidor: crear, arrancar, parar | ✅ |
| F4 | Lista maestra de jugadores | pendiente |
| Hito 2 | Minecraft Java con AuthMe para cuentas no premium | pendiente |

Ahora mismo soporta **Bedrock**. Java está previsto en el diseño: toda la parte
específica de una edición vive detrás de la interfaz `ServerFlavor`, así que
añadirlo es escribir un paquete hermano de `internal/adapter/bedrock`.

## Cómo funciona

```text
adapter/web ─┐
adapter/sqlite ┼──> app (casos de uso + puertos) ──> domain (reglas)
adapter/dockerx│
adapter/bedrock┘
```

Arquitectura hexagonal: `domain` no importa nada fuera de la librería estándar,
`app` declara los puertos que necesita, y los adaptadores los implementan. Las
dependencias apuntan siempre hacia adentro.

Detalles que quizá no esperas y están así a propósito:

- **Sin ORM.** SQL directo con migraciones de `goose`, y SQL portable para poder
  pasar a PostgreSQL sin reescribir el historial.
- **Sin framework de frontend.** `html/template` y CSS propio. La interfaz es un
  login y unas tablas; Angular o React aportarían más peso que utilidad.
- **Sin el SDK de Docker.** Se habla la API HTTP directamente: el SDK arrastraba
  267 paquetes, y en producción se va contra un `docker-socket-proxy` que expone
  exactamente esa misma API.
- **NBT little-endian propio.** El `level.dat` de Bedrock no es como el de Java:
  va sin comprimir, con cabecera de 8 bytes y little-endian. Casi todas las
  librerías asumen el formato de Java y devuelven basura sin avisar.

## Empezar

```bash
cp .env.example .env     # ajustar las rutas y el superusuario
go run ./cmd/mcvps -seed # crea el rol superusuario y su cuenta
go run ./cmd/mcvps       # arranca el panel
```

Hace falta un Docker accesible para la gestión de servidores. Sin él, el panel
arranca igual y sirve para mapas, usuarios y roles.

### Comandos

| Comando | Qué hace |
|---|---|
| `mcvps` | Arranca el panel y aplica las migraciones pendientes |
| `mcvps -seed` | Crea el rol y la cuenta de superusuario, si no existen |
| `mcvps -migrations` | Muestra el estado de las migraciones |
| `mcvps -new-migration "nombre"` | Crea el siguiente archivo de migración |
| `mcvps -healthcheck` | Sonda para el `HEALTHCHECK` de Docker |

## Despliegue

`docker compose up -d --build`, detrás de un reverse proxy con TLS.

Dos cosas que no son opcionales:

**El panel no publica puertos.** Va solo en la red del proxy. Publicar un puerto
en Docker puede saltarse las reglas de UFW por completo, así que un `ports:`
"para probar" deja el panel abierto a internet aunque el firewall diga lo
contrario.

**`MCVPS_GAME_HOST` no puede ser `localhost`** dentro del contenedor: el servidor
de Minecraft corre en otro contenedor, y ahí es donde se consulta el número de
jugadores conectados.

## Documentación

El diseño, las decisiones y los hallazgos de campo viven en un repositorio
aparte, enlazado como submódulo en `docs/`.

Es privado porque documenta una infraestructura concreta. Si te interesa el
razonamiento detrás de alguna decisión, abre un issue y lo explico.

## Licencia

Pendiente de decidir.
