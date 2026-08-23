// Package config lee la configuracion desde variables de entorno.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Direccion donde escucha el panel DENTRO del contenedor.
	// Nunca se publica al host: NPM llega por la red proxy (M-7).
	Addr string

	// Ruta del archivo SQLite (D-14).
	DBPath string

	// Carpeta donde se guardan los archivos de los mapas.
	MapsPath string

	// Tamano maximo de un mapa subido (D-11).
	MaxUpload int64

	// Carpeta donde vive el /data de cada instancia de servidor.
	InstancesPath string

	// Como se llega a Docker. En produccion apunta al docker-socket-proxy
	// (M-4), no al socket crudo.
	DockerHost string

	// Host donde responde el servidor de Minecraft, para el ping de jugadores.
	// Dentro del contenedor "localhost" no vale: el servidor corre en otro.
	GameHost string

	// Credenciales del superusuario. Solo se usan si todavia no existe: el
	// arranque nunca pisa una cuenta ya creada.
	SuperuserEmail    string
	SuperuserPassword string

	// Duracion de la sesion de panel.
	SessionTTL time.Duration

	// true cuando el panel se sirve por HTTPS (detras de NPM).
	// Marca la cookie como Secure.
	SecureCookies bool
}

// Load lee la configuracion. Si existe un archivo .env en el directorio de
// trabajo, lo carga primero; el entorno real siempre tiene prioridad.
func Load() (Config, error) {
	if err := LoadFile(".env"); err != nil {
		return Config{}, err
	}

	c := Config{
		Addr:          env("MCVPS_ADDR", ":8080"),
		DBPath:        env("MCVPS_DB_PATH", "/data/mcvps.db"),
		MapsPath:      env("MCVPS_MAPS_PATH", "/data/maps"),
		InstancesPath: env("MCVPS_INSTANCES_PATH", "/data/instances"),
		DockerHost:    os.Getenv("MCVPS_DOCKER_HOST"),
		GameHost:      env("MCVPS_GAME_HOST", "127.0.0.1"),
		MaxUpload:     1 << 30, // 1 GiB
		SuperuserEmail:    os.Getenv("MCVPS_SUPERUSER_EMAIL"),
		SuperuserPassword: os.Getenv("MCVPS_SUPERUSER_PASSWORD"),
		SessionTTL:    12 * time.Hour,
		SecureCookies: envBool("MCVPS_SECURE_COOKIES", true),
	}

	if v := os.Getenv("MCVPS_MAX_UPLOAD_MB"); v != "" {
		mb, err := strconv.ParseInt(v, 10, 64)
		if err != nil || mb <= 0 {
			return Config{}, fmt.Errorf("MCVPS_MAX_UPLOAD_MB invalido (%q)", v)
		}
		c.MaxUpload = mb << 20
	}

	if v := os.Getenv("MCVPS_SESSION_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("MCVPS_SESSION_TTL invalido (%q): %w", v, err)
		}
		c.SessionTTL = d
	}

	// El arranque del superusuario es todo o nada: media configuracion deja el
	// panel sin forma de entrar y sin decir por que.
	if (c.SuperuserEmail == "") != (c.SuperuserPassword == "") {
		return Config{}, errors.New("MCVPS_SUPERUSER_EMAIL y MCVPS_SUPERUSER_PASSWORD deben definirse juntos")
	}

	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
