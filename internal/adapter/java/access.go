package java

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Excelion29/mc-config/internal/app"
)

// entradaLista es una fila de whitelist.json.
//
// Java identifica por UUID, no por nombre. El nombre viaja igual porque el
// archivo se lee a ojo desde la VPS, pero al servidor le vale el UUID.
type entradaLista struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// entradaOp es una fila de ops.json.
type entradaOp struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
	// Level 4 es el maximo: permite todos los comandos, incluidos los de
	// administracion del servidor. Es lo que la gente entiende por "op".
	Level int `json:"level"`
	// BypassesPlayerLimit deja entrar aunque el servidor este lleno. Va en
	// false: quien administra no deberia ocupar la plaza de un amigo sin
	// haberlo decidido a proposito.
	BypassesPlayerLimit bool `json:"bypassesPlayerLimit"`
}

// WriteWhitelist escribe whitelist.json.
//
// Se escribe SIEMPRE, aunque este vacia. En Java eso importa menos que en
// Bedrock -alli una lista vacia bloquea a todos- pero deja claro que el panel
// gestiona el archivo y que no es que se haya perdido.
//
// Las entradas sin UUID se descartan: el servidor no sabria a quien se
// refieren. Que eso no pase es responsabilidad de quien da de alta, que
// resuelve el UUID en ese momento (H-J-8).
func WriteWhitelist(dataDir string, jugadores []app.PlayerRef) error {
	return escribirJSON(dataDir, "whitelist.json", conUUID(jugadores, func(r app.PlayerRef) any {
		return entradaLista{UUID: r.ID, Name: r.Name}
	}))
}

// WriteOps escribe ops.json.
func WriteOps(dataDir string, ops []app.PlayerRef) error {
	return escribirJSON(dataDir, "ops.json", conUUID(ops, func(r app.PlayerRef) any {
		return entradaOp{UUID: r.ID, Name: r.Name, Level: 4}
	}))
}

// conUUID filtra las referencias sin identificador y las transforma.
func conUUID(refs []app.PlayerRef, f func(app.PlayerRef) any) []any {
	out := make([]any, 0, len(refs))
	for _, r := range refs {
		if strings.TrimSpace(r.ID) == "" {
			continue
		}
		out = append(out, f(r))
	}
	return out
}

func escribirJSON(dataDir, nombre string, v []any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("generando %s: %w", nombre, err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, nombre), data, 0o644); err != nil {
		return fmt.Errorf("escribiendo %s: %w", nombre, err)
	}
	return nil
}

// OfflineUUID se mudo a domain.OfflineUUID.
//
// No era cosa del adaptador: no habla con nadie ni depende de Paper. Es la
// regla de como se identifica un jugador sin cuenta comprada, y la necesitan
// tambien el alta de jugadores y la escritura de la lista.
