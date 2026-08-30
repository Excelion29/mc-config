package web

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var (
	usoDeToken   = regexp.MustCompile(`var\(\s*(--[a-zA-Z0-9-]+)`)
	declaraToken = regexp.MustCompile(`(?m)^\s*(--[a-zA-Z0-9-]+)\s*:`)
)

// TestCadaVariableDeCssExiste caza los tokens inventados.
//
// Un var(--que-no-existe) NO da error en ningun sitio: el navegador se limita a
// no aplicar la propiedad. El resultado es una letra con el tamano por defecto o
// un hueco sin margen, que nadie relaciona con una errata en un nombre.
//
// Paso justo eso al escribir el panel de acceso: --texto-s en vez de
// --texto-sm. Compilaba, las pruebas pasaban, y la diferencia solo se veia
// mirando muy de cerca.
func TestCadaVariableDeCssExiste(t *testing.T) {
	hojas, err := fs.Glob(assets, "static/css/*.css")
	if err != nil {
		t.Fatal(err)
	}
	if len(hojas) == 0 {
		t.Fatal("no se encontro ninguna hoja de estilos")
	}

	declarados := map[string]bool{}
	usados := map[string][]string{}

	for _, hoja := range hojas {
		datos, err := fs.ReadFile(assets, hoja)
		if err != nil {
			t.Fatal(err)
		}
		css := string(datos)

		for _, m := range declaraToken.FindAllStringSubmatch(css, -1) {
			declarados[m[1]] = true
		}
		for _, m := range usoDeToken.FindAllStringSubmatch(css, -1) {
			nombre := strings.TrimPrefix(hoja, "static/css/")
			usados[m[1]] = append(usados[m[1]], nombre)
		}
	}

	var huerfanos []string
	for token := range usados {
		if !declarados[token] {
			huerfanos = append(huerfanos, token)
		}
	}
	sort.Strings(huerfanos)

	for _, token := range huerfanos {
		t.Errorf("%s se usa en %v y no esta declarado en ningun sitio",
			token, unicos(usados[token]))
	}
}

func unicos(xs []string) []string {
	visto := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !visto[x] {
			visto[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
