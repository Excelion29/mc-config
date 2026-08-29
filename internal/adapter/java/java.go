package java

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Excelion29/mc-config/internal/app"
	"github.com/Excelion29/mc-config/internal/domain"
)

// La misma imagen que Bedrock, en su variante de Java. Descarga sola el
// servidor de la version que se le pida, que es lo que necesita el panel.
// Confirmada a mano en F5.0 (ver 13-validacion-java.md).
const Image = "itzg/minecraft-server"

type Flavor struct {
	extractor app.WorldExtractor
}

func New(extractor app.WorldExtractor) *Flavor {
	return &Flavor{extractor: extractor}
}

func (*Flavor) Edition() domain.Edition { return domain.EditionJava }
func (*Flavor) DefaultPort() int        { return domain.PortJava }

func (*Flavor) Spec(inst *domain.Instance, dataDir string) app.ContainerSpec {
	version := inst.Version
	if version == "" {
		version = "LATEST"
	}

	return app.ContainerSpec{
		Name:  "mcvps-" + inst.Slug,
		Image: Image,
		Env: map[string]string{
			"EULA": "TRUE",
			// PAPER y no vanilla: es la plataforma de plugins, y AuthMe -lo
			// que permitira jugar a los amigos no premium- es un plugin
			// (D-15).
			"TYPE":    "PAPER",
			"VERSION": version,
			"MEMORY":  strconv.Itoa(inst.MemoryMB) + "M",
			// Lo decide el modo del panel. En modo normal autentica Mojang;
			// en modo sin conexion no, y de eso se encargan AuthMe y
			// FastLogin (D-07, D-17).
			"ONLINE_MODE": boolEnv(!inst.Auth.SinConexion()),
		},
		DataDir:  dataDir,
		PortHost: inst.Port,
		PortIn:   domain.PortJava,
		// La diferencia mas visible con Bedrock: Java va por TCP.
		Protocol: "tcp",
		MemoryMB: inst.MemoryMB,
		CPUs:     inst.CPUs,
	}
}

// InstallWorld deja el mundo en la carpeta que indica level-name.
//
// Si no hay archivo, no hay nada que instalar: el mundo se creo vacio y lo
// genera el propio servidor al arrancar, a partir de la semilla que va en
// server.properties.
func (f *Flavor) InstallWorld(archivePath, dataDir, levelName string) error {
	if archivePath == "" {
		return nil
	}

	dest := filepath.Join(dataDir, levelName)
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("limpiando %s: %w", dest, err)
	}
	return f.extractor.Extract(archivePath, dest)
}

// WriteConfig genera server.properties, whitelist.json y ops.json.
func (f *Flavor) WriteConfig(inst *domain.Instance, dataDir string, permitidos []app.PlayerRef) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("creando %s: %w", dataDir, err)
	}

	props := map[string]string{
		"server-port": strconv.Itoa(domain.PortJava),
		"level-name":  inst.LevelName,
		"motd":        inst.Name,
		"online-mode": boolProp(!inst.Auth.SinConexion()),
		// enforce-secure-profile tiene que APAGARSE en modo sin conexion: exige
		// que el cliente traiga una firma de Mojang, y una cuenta no premium no
		// la tiene. Con esto puesto, los no premium no entran ni con AuthMe, y
		// el rechazo no menciona esta clave por ningun sitio.
		"enforce-secure-profile": boolProp(!inst.Auth.SinConexion()),

		// La whitelist va SIEMPRE puesta, y con las DOS claves.
		//
		// En Java viene apagada de fabrica, al reves que en Bedrock: sin ella
		// entra cualquiera con cuenta de Mojang, y el 25565 es el puerto que
		// escanean sin parar (H-J-3).
		//
		// enforce-whitelist ademas echa a quien ya este dentro cuando deja de
		// estar en la lista. Con solo white-list, quitas a alguien y sigue
		// jugando hasta que se desconecte solo (H-J-11).
		"white-list":        "true",
		"enforce-whitelist": "true",

		// --- Reglas: se reescriben en cada arranque ---
		"gamemode":    string(inst.Rules.Gamemode),
		"difficulty":  string(inst.Rules.Difficulty),
		"pvp":         boolProp(inst.Rules.PvP),
		"max-players": strconv.Itoa(inst.Rules.MaxPlayers),
		// En Java los comandos no tienen interruptor: dependen de ser
		// operador. Lo mas parecido es permitir bloques de comandos, que es lo
		// que cambia si el mundo "admite comandos" o no.
		"enable-command-block": boolProp(inst.Rules.AllowCommands),

		// RCON escucha en 0.0.0.0 dentro del contenedor. NO se publica hacia
		// fuera -el Spec solo abre el puerto del juego- pero se deja apagado
		// mientras no se use: un RCON accesible es control total del servidor
		// (H-J-4).
		"enable-rcon": "false",

		// Interfaz de administracion nueva en 26.2, apagada de fabrica. Se
		// queda apagada: un puerto mas con TLS y un secreto es superficie que
		// no necesitamos (H-J-12).
		"management-server-enabled": "false",
	}

	// --- Paquete de texturas ---
	//
	// Se sirve por ENLACE: la clave lleva una URL y es el cliente quien la
	// descarga al conectarse. El panel no aloja el archivo (M-2).
	//
	// Las claves solo se escriben si hay paquete. Dejar "resource-pack" vacio
	// no es lo mismo que no ponerlo en algunas versiones, y no hace falta
	// averiguar en cuales.
	if inst.Pack.URL != "" {
		props["resource-pack"] = inst.Pack.URL

		// Sin el hash el cliente vuelve a descargar el paquete ENTERO en cada
		// conexion, porque no tiene forma de saber que el que guarda es el
		// mismo. Va vacio cuando no se pudo calcular, y entonces mejor no
		// escribir la clave que escribir un hash falso: uno que no cuadra hace
		// que el cliente rechace el paquete.
		if inst.Pack.SHA1 != "" {
			props["resource-pack-sha1"] = inst.Pack.SHA1
		}

		// Echa a quien lo rechace. Lo decide cada mundo.
		props["require-resource-pack"] = boolProp(inst.Pack.Required)
	}

	// --- Generacion: solo si el mundo nacio vacio ---
	if inst.Gen.Seed != "" {
		props["level-seed"] = inst.Gen.Seed
	}
	props["generate-structures"] = boolProp(inst.Gen.Structures)
	if t := levelType(inst.Gen.LevelType); t != "" {
		props["level-type"] = t
	}

	if err := writeProperties(dataDir, props); err != nil {
		return err
	}
	return WriteWhitelist(dataDir, permitidos)
}

// ReloadAllowlist recarga la whitelist sin reiniciar.
func (*Flavor) ReloadAllowlist(ctx context.Context, rt app.ContainerRuntime, containerID string) error {
	if containerID == "" {
		return nil
	}
	return rt.SendStdin(ctx, containerID, "whitelist reload")
}

func (*Flavor) WritePermissions(dataDir string, ops []app.PlayerRef) error {
	return WriteOps(dataDir, ops)
}

// ReloadPermissions aplica ops.json sin reiniciar.
func (*Flavor) ReloadPermissions(ctx context.Context, rt app.ContainerRuntime, containerID string) error {
	if containerID == "" {
		return nil
	}
	// Java no tiene un "reload" de ops: se recarga junto con la whitelist al
	// releer los archivos de acceso.
	return rt.SendStdin(ctx, containerID, "whitelist reload")
}

// boolProp escribe un booleano como lo espera server.properties.
func boolProp(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// levelType traduce el tipo de terreno al valor que espera Java.
//
// Java los nombra con espacio de nombres -"minecraft:flat"- y los dos puntos
// van escapados al escribir el archivo, de lo que ya se encarga
// escapePropertyValue. Devuelve vacio para los que no conoce: mejor no
// escribir la clave que escribirla con algo que el servidor ignora.
func levelType(t domain.LevelType) string {
	switch t {
	case domain.LevelFlat:
		return "minecraft:flat"
	case domain.LevelLargeBiomes:
		return "minecraft:large_biomes"
	case domain.LevelAmplified:
		return "minecraft:amplified"
	case domain.LevelNormal:
		return "minecraft:normal"
	}
	return ""
}

// boolEnv escribe un booleano como lo esperan las variables de la imagen, que
// las quiere en mayusculas.
func boolEnv(b bool) string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}
