# --- Compilacion ------------------------------------------------------------
FROM golang:1.26-alpine AS build

WORKDIR /src

# Las dependencias se copian aparte para que su capa se cachee y no haya que
# volver a descargarlas cada vez que cambia el codigo.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produce un binario estatico. Es posible porque la base de datos
# usa modernc.org/sqlite, en Go puro (D-12). Con mattn/go-sqlite3 haria falta
# cgo y no se podria usar una imagen distroless.
# -trimpath quita las rutas de compilacion; -s -w quita simbolos de depuracion.
RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath -ldflags="-s -w" \
        -o /out/mcvps ./cmd/mcvps

# El directorio de datos se crea aqui con el dueno correcto. Docker inicializa
# el volumen con nombre a partir de la imagen, asi que hereda estos permisos y
# el usuario sin privilegios puede escribir sin tener que hacer chown al vuelo.
RUN mkdir -p /data && chown 65532:65532 /data

# --- Ejecucion --------------------------------------------------------------
# distroless: solo el binario y los certificados. Sin shell, sin gestor de
# paquetes, sin utilidades. Importa especialmente aqui, porque compartimos
# maquina con la produccion de un cliente (D-09).
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/mcvps /mcvps
COPY --from=build --chown=65532:65532 /data /data

USER 65532:65532
WORKDIR /data

ENV MCVPS_ADDR=":8080" \
    MCVPS_DB_PATH="/data/mcvps.db"

EXPOSE 8080

# Sin shell no hay curl ni wget: el binario se sondea a si mismo.
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/mcvps", "-healthcheck"]

ENTRYPOINT ["/mcvps"]
