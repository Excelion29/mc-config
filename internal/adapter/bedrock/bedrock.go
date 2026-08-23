// Package bedrock implementa app.ServerFlavor para Minecraft Bedrock.
//
// Todo lo especifico de esta edicion vive aqui: la imagen, los puertos, el
// formato de server.properties y de allowlist.json. Anadir Java en el hito 2
// (D-01) es escribir un paquete hermano que implemente la misma interfaz.
package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Excelion29/mc-config/internal/app"
	"github.com/Excelion29/mc-config/internal/domain"
)

// Imagen confirmada en F0: descarga sola el binario del servidor de la version
// que se le pida por VERSION, que es justo lo que necesita el panel.
const Image = "itzg/minecraft-bedrock-server"

type Flavor struct {
	extractor app.WorldExtractor
}

func New(extractor app.WorldExtractor) *Flavor {
	return &Flavor{extractor: extractor}
}

func (*Flavor) Edition() domain.Edition { return domain.EditionBedrock }
func (*Flavor) DefaultPort() int        { return domain.PortBedrock }

func (*Flavor) Spec(inst *domain.Instance, dataDir string) app.ContainerSpec {
	version := inst.Version
	if version == "" {
		version = "LATEST"
	}

	return app.ContainerSpec{
		Name:  "mcvps-" + inst.Slug,
		Image: Image,
		Env: map[string]string{
			"EULA":        "TRUE",
			"VERSION":     version,
			"SERVER_NAME": inst.Name,
			"LEVEL_NAME":  inst.LevelName,
			// Se exige cuenta comprada: en Bedrock remoto no hay alternativa,
			// Mojang lo impone (D-07).
			"ONLINE_MODE": "true",
			"ALLOW_LIST":  "true",
		},
		DataDir:  dataDir,
		PortHost: inst.Port,
		PortIn:   domain.PortBedrock,
		Protocol: "udp",
		MemoryMB: inst.MemoryMB,
		CPUs:     inst.CPUs,
	}
}

// InstallWorld deja el mundo en worlds/<levelName>, que es donde el servidor lo
// busca segun level-name (confirmado en F0.3).
func (f *Flavor) InstallWorld(archivePath, dataDir, levelName string) error {
	dest := filepath.Join(dataDir, "worlds", levelName)

	// Se limpia primero: reinstalar sobre restos de otro mundo mezcla dos
	// bases de datos LevelDB y el servidor no arranca.
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("limpiando %s: %w", dest, err)
	}
	return f.extractor.Extract(archivePath, dest)
}

// WriteConfig genera server.properties y allowlist.json.
func (f *Flavor) WriteConfig(inst *domain.Instance, dataDir string, allowlist []string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("creando %s: %w", dataDir, err)
	}

	props := map[string]string{
		"server-name":   inst.Name,
		"level-name":    inst.LevelName,
		"server-port":   strconv.Itoa(domain.PortBedrock),
		"server-portv6": strconv.Itoa(domain.PortBedrock + 1),
		"online-mode":   "true",
		"allow-list":    "true",
		"gamemode":      "survival",
		"difficulty":    "easy",
		"max-players":   "12",
	}

	if err := writeProperties(filepath.Join(dataDir, "server.properties"), props); err != nil {
		return err
	}
	return WriteAllowlist(dataDir, allowlist)
}

// WriteAllowlist escribe allowlist.json con el formato exacto que genera el
// servidor, confirmado en F0.4 (H-F0-7):
//
//	[{"ignoresPlayerLimit":false,"name":"Gamertag"}]
//
// Se escribe SIEMPRE, aunque la lista este vacia. Un archivo ausente con
// allow-list=true tambien bloquea a todos, pero sin dejar rastro de por que.
func WriteAllowlist(dataDir string, names []string) error {
	type entry struct {
		IgnoresPlayerLimit bool   `json:"ignoresPlayerLimit"`
		Name               string `json:"name"`
	}

	list := make([]entry, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		list = append(list, entry{Name: n})
	}

	data, err := json.Marshal(list)
	if err != nil {
		return fmt.Errorf("generando allowlist.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "allowlist.json"), data, 0o644); err != nil {
		return fmt.Errorf("escribiendo allowlist.json: %w", err)
	}
	return nil
}

// ReloadAllowlist recarga la lista sin reiniciar el servidor.
//
// Requiere `exec` sobre el contenedor, lo que obliga a matizar M-4: el proxy de
// Docker no puede prohibir exec del todo. Debe permitirlo acotado a
// send-command y solo sobre contenedores de MCVPS, nunca sobre los del cliente.
func (*Flavor) ReloadAllowlist(ctx context.Context, rt app.ContainerRuntime, containerID string) error {
	if containerID == "" {
		return nil
	}
	_, err := rt.Exec(ctx, containerID, []string{"send-command", "allowlist", "reload"})
	return err
}

// Players consulta el servidor con un ping de RakNet.
func (*Flavor) Players(ctx context.Context, host string, port int) (int, int, error) {
	info, err := Ping(ctx, host, port, 3*time.Second)
	if err != nil {
		return 0, 0, err
	}
	return info.Online, info.Max, nil
}

// writeProperties escribe un archivo clave=valor.
//
// Si ya existe, se conservan las lineas que no gestionamos: el servidor genera
// decenas de opciones y pisarlas todas borraria lo que alguien hubiera ajustado
// a mano.
func writeProperties(path string, values map[string]string) error {
	existing := map[string]string{}
	var order []string

	if raw, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			key, value, ok := strings.Cut(trimmed, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			if _, seen := existing[key]; !seen {
				order = append(order, key)
			}
			existing[key] = strings.TrimSpace(value)
		}
	}

	for k, v := range values {
		if _, seen := existing[k]; !seen {
			order = append(order, k)
		}
		existing[k] = v
	}

	var b strings.Builder
	b.WriteString("# Generado por MCVPS. Las claves que gestiona el panel se\n")
	b.WriteString("# sobrescriben en cada arranque; el resto se conserva.\n")
	for _, k := range order {
		fmt.Fprintf(&b, "%s=%s\n", k, existing[k])
	}

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("escribiendo server.properties: %w", err)
	}
	return nil
}
