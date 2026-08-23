package java

import (
	"crypto/md5"
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

// OfflineUUID calcula el UUID que Minecraft asigna a un jugador SIN cuenta
// premium, a partir de su nombre.
//
// No hay nada que consultar: es determinista. Java toma el MD5 de la cadena
// "OfflinePlayer:<nombre>" y lo convierte en un UUID de version 3. Por eso un
// jugador no premium tiene siempre el mismo UUID en cualquier servidor sin
// conexion, y por eso su nombre es su identidad -y por eso hace falta AuthMe
// para que nadie use el nombre de otro (D-07)-.
//
// Esto solo aplica con online-mode=false, o sea a partir de F6. Para cuentas
// premium el UUID lo asigna Mojang y hay que preguntarselo.
//
// PENDIENTE DE VERIFICAR contra un servidor real cuando se monte AuthMe: el
// algoritmo esta documentado, pero en este proyecto no damos por buena una
// suposicion hasta verla funcionar.
func OfflineUUID(nombre string) string {
	suma := md5.Sum([]byte("OfflinePlayer:" + nombre))

	// Se marcan version 3 y variante RFC 4122, que es lo que hace la funcion
	// equivalente de Java al construir el UUID desde unos bytes.
	b := suma
	b[6] = (b[6] & 0x0f) | 0x30
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
