package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var llamadaFragmento = regexp.MustCompile(`fragment\(w,\s*"([^"]+)",\s*"([^"]+)"`)

// TestLosFragmentosNoMiranLaPagina protege un fallo que rompio /players entero.
//
// Un bloque que se pinta suelto -para actualizar una fila sin recargar- recibe
// SOLO su dato. Dentro de una plantilla, "$" es su propio argumento, no la
// pagina: si el bloque escribe $.Algo, al pintarlo suelto ese campo no existe.
//
// Y no falla solo el fragmento. La pagina completa usa el mismo bloque, asi que
// tumba las dos vias a la vez y la pantalla se queda en "error al generar la
// pagina", sin decir cual de las cincuenta lineas fue.
//
// La regla es simple: lo que la fila necesite, se le da resuelto desde Go.
func TestLosFragmentosNoMiranLaPagina(t *testing.T) {
	fuentes, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	// Que bloque de que plantilla se pinta suelto, segun los propios handlers.
	sueltos := map[string][]string{}
	for _, f := range fuentes {
		datos, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range llamadaFragmento.FindAllStringSubmatch(string(datos), -1) {
			sueltos[m[1]] = append(sueltos[m[1]], m[2])
		}
	}

	if len(sueltos) == 0 {
		t.Fatal("no se encontro ninguna llamada a fragment; la prueba no comprueba nada")
	}

	for plantilla, bloques := range sueltos {
		datos, err := fs.ReadFile(assets, "templates/"+plantilla)
		if err != nil {
			t.Errorf("%s se pinta suelta pero no existe: %v", plantilla, err)
			continue
		}

		for _, bloque := range bloques {
			cuerpo, ok := bloqueDefinido(string(datos), bloque)
			if !ok {
				t.Errorf("%s: se pinta el bloque %q y no esta definido", plantilla, bloque)
				continue
			}
			if strings.Contains(cuerpo, "$.") {
				t.Errorf("%s, bloque %q: usa $. y se pinta suelto, "+
					"donde $ es su propio dato y no la pagina", plantilla, bloque)
			}
		}
	}
}

// bloqueDefinido devuelve el cuerpo de un {{define "nombre"}} ... {{end}}.
//
// Se cuentan las aperturas porque dentro hay if y range con sus propios {{end}}:
// parar en el primero daria un trozo, y justo la parte que falta es el final,
// que es donde suelen ir las acciones.
func bloqueDefinido(html, nombre string) (string, bool) {
	marca := `{{define "` + nombre + `"}}`
	i := strings.Index(html, marca)
	if i < 0 {
		return "", false
	}
	resto := html[i+len(marca):]

	hondura := 1
	for j := 0; j < len(resto); j++ {
		switch {
		case strings.HasPrefix(resto[j:], "{{if"),
			strings.HasPrefix(resto[j:], "{{range"),
			strings.HasPrefix(resto[j:], "{{with"),
			strings.HasPrefix(resto[j:], "{{define"),
			strings.HasPrefix(resto[j:], "{{block"):
			hondura++
		case strings.HasPrefix(resto[j:], "{{end}}"):
			hondura--
			if hondura == 0 {
				return resto[:j], true
			}
		}
	}
	return resto, true
}
