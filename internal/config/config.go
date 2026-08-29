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
	WorldsPath string
	// PluginsPath es la cache de complementos de servidor. Compartida entre
	// instancias: el mismo .jar sirve para todas.
	PluginsPath string

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
	// GameNetwork es la red de Docker que comparten el panel y las instancias.
	//
	// Con ella, el panel pregunta a cada servidor por su NOMBRE de contenedor
	// y no hace falta ni NAT ni salir al host. Vacia, las instancias van a la
	// red por defecto y el panel no las alcanza, porque Docker aisla las redes
	// de usuario entre si.
	GameNetwork string

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
		Addr:   env("MCVPS_ADDR", ":8080"),
		DBPath: env("MCVPS_DB_PATH", "/data/mcvps.db"),
		// Se acepta el nombre viejo a proposito. La variable ya esta puesta
		// en el .env de la VPS, y el despliegue es automatico al empujar a
		// main: cambiarle el nombre sin mas dejaria el panel apuntando al
		// valor por defecto -otra carpeta- y los mundos "desaparecerian" sin
		// un solo error en el log. Se retirara cuando el .env se actualice.
		WorldsPath:        envAlguno("/data/worlds", "MCVPS_WORLDS_PATH", "MCVPS_MAPS_PATH"),
		PluginsPath:       env("MCVPS_PLUGINS_PATH", "/data/plugins"),
		InstancesPath:     env("MCVPS_INSTANCES_PATH", "/data/instances"),
		DockerHost:        os.Getenv("MCVPS_DOCKER_HOST"),
		GameHost:          env("MCVPS_GAME_HOST", "127.0.0.1"),
		GameNetwork:       env("MCVPS_GAME_NETWORK", ""),
		MaxUpload:         1 << 30, // 1 GiB
		SuperuserEmail:    os.Getenv("MCVPS_SUPERUSER_EMAIL"),
		SuperuserPassword: os.Getenv("MCVPS_SUPERUSER_PASSWORD"),
		SessionTTL:        12 * time.Hour,
		SecureCookies:     envBool("MCVPS_SECURE_COOKIES", true),
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

// envAlguno devuelve la primera variable de entorno con valor, o el valor por
// defecto. Sirve para renombrar una variable sin romper los despliegues que
// aun usan el nombre anterior.
func envAlguno(porDefecto string, nombres ...string) string {
	for _, n := range nombres {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return porDefecto
}
