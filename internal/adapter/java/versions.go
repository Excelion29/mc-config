package java

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Excelion29/mc-config/internal/app"
)

// PaperMC si publica un historico completo, al contrario que Mojang con
// Bedrock.
//
// OJO con el endpoint: la v2 de api.papermc.io esta RETIRADA -devuelve 410 con
// un "sunset"- y la v3 en ese mismo dominio contesta 403. El bueno es otro
// dominio. Se comprobo a mano antes de escribir esto, porque la documentacion
// que uno recuerda envejece.
const versionsAPI = "https://fill.papermc.io/v3/projects/paper"

type cacheVersiones struct {
	mu       sync.Mutex
	opciones []app.VersionOption
	traido   time.Time
	ttl      time.Duration
}

// Media hora, igual que Bedrock. Paper publica varias compilaciones al dia,
// pero las VERSIONES nuevas salen cada muchas semanas: preguntar mas a menudo
// solo gasta.
var cache = &cacheVersiones{ttl: 30 * time.Minute}

// AvailableVersions devuelve las versiones instalables de Paper.
//
// Si la API no responde se devuelve solo LATEST, que es la palabra clave que
// entiende la imagen. Una lista corta es mejor que un error que impida crear
// nada: el panel tiene que seguir funcionando aunque PaperMC este caido.
func (*Flavor) AvailableVersions(ctx context.Context) ([]app.VersionOption, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if time.Since(cache.traido) < cache.ttl && len(cache.opciones) > 0 {
		return cache.opciones, nil
	}

	opciones, err := pedirVersiones(ctx)
	if err != nil {
		return soloLatest(), nil
	}

	cache.opciones = opciones
	cache.traido = time.Now()
	return opciones, nil
}

func pedirVersiones(ctx context.Context) ([]app.VersionOption, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionsAPI, nil)
	if err != nil {
		return nil, err
	}

	// Tiempo limite corto: esto se consulta al pintar una pagina, y nadie
	// deberia esperar cinco segundos a que responda un tercero.
	cliente := &http.Client{Timeout: 5 * time.Second}
	resp, err := cliente.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("papermc respondio %d", resp.StatusCode)
	}

	versiones, err := leerVersiones(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(versiones) == 0 {
		return nil, fmt.Errorf("papermc no devolvio ninguna version")
	}
	return construirOpciones(versiones), nil
}

// leerVersiones extrae la lista respetando el ORDEN que da la API.
//
// La respuesta agrupa por familia y ya viene de mas nueva a mas vieja:
//
//	{"versions": {"26.2": ["26.2", "26.2-rc-2"], "26.1": [...], "1.21": [...]}}
//
// Decodificarlo a un map de Go perderia ese orden, porque en Go recorrer un
// map da un orden distinto cada vez. Por eso se lee con el decodificador por
// piezas: es la unica forma de conservar el orden de las claves.
//
// Y conservarlo importa: NO se interpreta el formato de la version. El esquema
// paso de "1.21.x" a "26.2" sin avisar (H-J-5), asi que ordenar por numero
// seria apostar sobre lo unico que ya demostro que cambia. Quien sabe cual es
// la mas nueva es PaperMC.
func leerVersiones(r io.Reader) ([]familiaVersiones, error) {
	var raiz struct {
		Versions json.RawMessage `json:"versions"`
	}
	if err := json.NewDecoder(r).Decode(&raiz); err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(raiz.Versions))
	if _, err := dec.Token(); err != nil { // abre el objeto
		return nil, err
	}

	var out []familiaVersiones
	for dec.More() {
		nombre, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var versiones []string
		if err := dec.Decode(&versiones); err != nil {
			return nil, err
		}
		out = append(out, familiaVersiones{
			Nombre:    fmt.Sprint(nombre),
			Versiones: versiones,
		})
	}
	return out, nil
}

// familiaVersiones es un grupo tal como lo devuelve la API: "1.21" con todas
// las 1.21.x dentro.
type familiaVersiones struct {
	Nombre    string
	Versiones []string
}

// construirOpciones se queda con las estables, en el orden que dio la API.
//
// NO se recorta la lista. Se intento con un tope de doce y estaba mal: alguien
// puede tener un mundo de una version vieja, o unos amigos que no se han
// actualizado. Esconderlas obligaria a escribirlas a mano justo cuando mas
// falta hacen. Se ofrecen todas, agrupadas por familia para poder recorrerlas.
func construirOpciones(familias []familiaVersiones) []app.VersionOption {
	opciones := []app.VersionOption{{
		Value:       "LATEST",
		Label:       "La mas reciente (recomendado)",
		Recommended: true,
	}}

	primera := true
	for _, f := range familias {
		for _, v := range f.Versiones {
			if !esEstable(v) {
				continue
			}
			o := app.VersionOption{Value: v, Label: v, Group: f.Nombre}
			if primera {
				o.Note = "la ultima estable"
				primera = false
			}
			opciones = append(opciones, o)
		}
	}
	return opciones
}

// esEstable descarta candidatas y prelanzamientos.
//
// PaperMC si compila para "26.2-rc-2" o "1.21.11-pre5", pero eso no es una
// version estable: cambia de una semana a otra y los plugins no la soportan.
// Es el mismo motivo por el que no se ofrecen snapshots (H-J-6).
//
// La regla es que una version estable no lleva guion. Vale para los dos
// esquemas de numeracion que hemos visto, el viejo y el nuevo, precisamente
// porque no mira los numeros.
func esEstable(v string) bool { return !strings.Contains(v, "-") }

func soloLatest() []app.VersionOption {
	return []app.VersionOption{{
		Value:       "LATEST",
		Label:       "La mas reciente",
		Note:        "no se pudo consultar la lista de versiones",
		Recommended: true,
	}}
}
