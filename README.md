# MCVPS

Panel web para gestionar servidores de Minecraft en una VPS: preparar un mundo,
arrancarlo y decidir quién entra, todo desde el navegador y sin tocar la
consola.

Soporta **Bedrock** y **Java**. Escrito en Go, sin frontend aparte: un único
binario que sirve la interfaz y habla con Docker.

## Qué hace

- **Mundos.** Importar un mapa (`.mcworld` o `.zip`) o **crear uno nuevo** desde
  una semilla, en cualquiera de las dos ediciones. De un archivo importado se
  deduce sola la edición y la versión.
- **Servidores.** Crear, arrancar y parar. Sólo uno encendido a la vez: arrancar
  otro apaga el actual, avisando de cuánta gente va a desconectar.
- **Jugadores.** Una sola lista para todos los servidores. Se da de alta a
  alguien una vez y entra en todos los mundos, presentes y futuros, sin
  reiniciar nada.
- **Consola en vivo** de cada servidor, dentro de la propia página.
- **Usuarios y roles** del panel, con permisos editables y jerarquía por
  niveles.
- **Acceso.** Un interruptor decide si hace falta tener Minecraft comprado. Al
  abrirlo, el panel instala AuthMe y FastLogin él solo, y se niega a abrirlo si
  no están: sin ellos, ese modo deja entrar a cualquiera con el nombre que
  quiera.
- **Paquetes de texturas** por enlace, compartidos entre mundos. El panel guarda
  la URL, no el archivo: Java se lo pide al jugador al entrar.
- **Registro** de todo lo que se hace, con filtros y paginación.

## Estado

| Fase | Qué hace | |
|---|---|---|
| F1 | Autenticación, RBAC con jerarquía, registro de acciones | ✅ |
| F2 | Biblioteca de mundos: subida, validación y detección de edición/versión | ✅ |
| F3 | Instancias de servidor: crear, arrancar, parar | ✅ |
| F4 | Lista maestra de jugadores, propagándose a cada servidor | ✅ |
| F5 | **Minecraft Java** con Paper: mundos, whitelist, ops, consola | ✅ |
| F6 | AuthMe + FastLogin para cuentas no premium | ✅ |
| F7 | Paquetes de texturas por enlace, compartidos entre mundos | ✅ |

## Cómo funciona

```text
adapter/web ────┐
adapter/sqlite  │
adapter/dockerx ┼──> app (casos de uso + puertos) ──> domain (reglas)
adapter/bedrock │
adapter/java    │
adapter/mojang ─┘
```

Arquitectura hexagonal: `domain` no importa nada fuera de la librería estándar,
`app` declara los puertos que necesita, y los adaptadores los implementan. Las
dependencias apuntan siempre hacia adentro.

Y no es una intención, es una prueba: `internal/arch` comprueba en cada
ejecución que el dominio no dependa de nadie, que `app` sólo mire a `domain`,
que los adaptadores no se conozcan entre sí y que sólo `cmd/` elija
implementaciones concretas. Un import de más compila igual de bien; para cuando
se nota ya hay diez.

Añadir una edición es escribir un `ServerFlavor` y registrarlo en `cmd/mcvps`.
Java entró así, sin cambiar un solo caso de uso.

### Decisiones que quizá no esperas

- **Sin ORM.** SQL directo con migraciones de `goose`, y SQL portable para poder
  pasar a PostgreSQL sin reescribir el historial.
- **Sin framework de frontend.** `html/template`, CSS propio en capas —tokens,
  átomos, moléculas, organismos, layout— y unas 150 líneas de JavaScript para
  actualizar una fila sin recargar. Usan los atributos de HTMX (`hx-post`,
  `hx-target`) y su misma cabecera, así que sustituirlo por HTMX de verdad es
  borrar un archivo: ninguna plantilla cambia.
- **Sin el SDK de Docker.** Se habla la API HTTP directamente: el SDK arrastraba
  267 paquetes, y en producción se va contra un `docker-socket-proxy` que expone
  exactamente esa misma API.
- **NBT little-endian propio.** El `level.dat` de Bedrock no es como el de Java:
  va sin comprimir, con cabecera de 8 bytes y little-endian. Casi todas las
  librerías asumen el formato de Java y devuelven basura sin avisar.
- **Los dos protocolos de consulta, a mano.** Bedrock usa RakNet, dos datagramas
  UDP. Java es una conversación TCP con paquetes con longitud, enteros de tamaño
  variable y una respuesta en JSON. No se parecen en nada, y ninguno justifica
  una dependencia.

## Empezar

```bash
cp .env.example .env     # ajustar rutas y superusuario
go run ./cmd/mcvps -seed # crea el rol superusuario y su cuenta
go run ./cmd/mcvps       # arranca el panel
```

Hace falta un Docker accesible para la gestión de servidores. Sin él, el panel
arranca igual y sirve para mundos, jugadores, usuarios y roles.

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

Tres cosas que no son opcionales, y las tres se aprendieron rompiéndolas:

**El panel no publica puertos.** Va sólo en la red del proxy. Publicar un puerto
en Docker puede saltarse las reglas de UFW por completo —el tráfico se traduce
por DNAT y pasa por `FORWARD`, donde UFW ni se evalúa—, así que un `ports:`
"para probar" deja el panel abierto a internet aunque el firewall diga lo
contrario.

**`MCVPS_GAME_NETWORK` tiene que estar puesta.** Es la red que comparten el
panel y los servidores que crea, y es lo que le permite preguntarles por su
nombre de contenedor. Sin ella las instancias caen en la red por defecto y
quedan **inalcanzables** para el panel: Docker aísla las redes de usuario entre
sí. El síntoma es un servidor que arranca perfectamente y un panel clavado en
"arrancando", sin nada raro en el log del servidor, porque el servidor está
bien.

**Los servidores corren con el mismo usuario que el panel.** Las imágenes de
Minecraft se apropian de su carpeta de datos al arrancar, cada una con un
usuario distinto. Si no coincide con el del panel, éste deja de poder escribir
la configuración y la lista de permitidos — y no da error, porque cree que
escribió. El panel lo resuelve pasando su propio UID al contenedor.

## Documentación

El diseño, las decisiones y los hallazgos de campo viven en un repositorio
aparte, enlazado como submódulo en `docs/`. Son 16 decisiones numeradas y 25
hallazgos de campo, cada uno con lo que se probó y con lo que resultó ser
mentira.

Es privado porque documenta una infraestructura concreta. Si te interesa el
razonamiento detrás de alguna decisión, abre un issue y lo explico.

## Licencia

Pendiente de decidir.
