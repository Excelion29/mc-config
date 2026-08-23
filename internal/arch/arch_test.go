package arch

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const modulo = "github.com/Excelion29/mc-config/"

// raiz sube hasta la carpeta del modulo: la prueba corre dentro de
// internal/arch y tiene que mirar todo el arbol, no solo el suyo.
func raiz(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// importsDe devuelve, por paquete propio, los paquetes propios que importa.
// Los de la biblioteca estandar y las dependencias externas se descartan: la
// regla que se vigila aqui es la de las capas de este proyecto.
func importsDe(t *testing.T) map[string]map[string]string {
	t.Helper()
	base := raiz(t)
	fset := token.NewFileSet()
	grafo := map[string]map[string]string{}

	err := filepath.Walk(base, func(ruta string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if nombre := info.Name(); nombre == ".git" || nombre == "docs" || nombre == "secrets" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(ruta, ".go") || strings.HasSuffix(ruta, "_test.go") {
			return nil
		}

		rel, err := filepath.Rel(base, filepath.Dir(ruta))
		if err != nil {
			return err
		}
		paquete := filepath.ToSlash(rel)

		archivo, err := parser.ParseFile(fset, ruta, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		if grafo[paquete] == nil {
			grafo[paquete] = map[string]string{}
		}
		for _, imp := range archivo.Imports {
			destino, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if strings.HasPrefix(destino, modulo) {
				// Se guarda el archivo que lo trae, para que el fallo diga
				// donde mirar y no solo que la regla se rompio.
				grafo[paquete][strings.TrimPrefix(destino, modulo)] = filepath.Base(ruta)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return grafo
}

// TestDomainNoDependeDeNadie es la regla de la que cuelgan las demas.
//
// El dominio son las cosas del problema -un jugador, una instancia, un rol- y
// las reglas que las gobiernan. En cuanto importa una base de datos o un
// framework, esas reglas dejan de poder probarse solas y dejan de poder
// reusarse en otra parte.
func TestDomainNoDependeDeNadie(t *testing.T) {
	for destino, archivo := range importsDe(t)["internal/domain"] {
		t.Errorf("internal/domain importa %s (en %s): el dominio no debe depender de ninguna capa",
			destino, archivo)
	}
}

// TestAppSoloDependeDeDomain: los casos de uso declaran QUE necesitan mediante
// interfaces -los puertos- y nunca COMO se resuelve.
//
// Si app importara sqlite, cambiar de base de datos obligaria a tocar los
// casos de uso, que es exactamente lo que esta arquitectura evita.
func TestAppSoloDependeDeDomain(t *testing.T) {
	for destino, archivo := range importsDe(t)["internal/app"] {
		if destino != "internal/domain" {
			t.Errorf("internal/app importa %s (en %s): solo puede depender de internal/domain",
				destino, archivo)
		}
	}
}

// TestLosAdaptadoresNoSeConocen mantiene los adaptadores intercambiables.
//
// Si el adaptador web llamara directamente a sqlite, la web dejaria de poder
// probarse sin base de datos y sqlite no se podria sustituir sin tocar la web.
// Se hablan a traves de los puertos o no se hablan.
func TestLosAdaptadoresNoSeConocen(t *testing.T) {
	for paquete, imports := range importsDe(t) {
		if !strings.HasPrefix(paquete, "internal/adapter/") {
			continue
		}
		for destino, archivo := range imports {
			if strings.HasPrefix(destino, "internal/adapter/") && destino != paquete {
				t.Errorf("%s importa %s (en %s): los adaptadores no deben conocerse entre si",
					paquete, destino, archivo)
			}
		}
	}
}

// TestSoloLaRaizDeComposicionConoceLosAdaptadores.
//
// Alguien tiene que elegir que implementacion concreta se usa, y ese sitio es
// cmd/mcvps: es el unico punto del programa donde SQLite, Docker y el servidor
// web existen a la vez. Concentrarlo ahi es lo que permite que el resto del
// codigo no sepa nada de ellos.
func TestSoloLaRaizDeComposicionConoceLosAdaptadores(t *testing.T) {
	for paquete, imports := range importsDe(t) {
		if strings.HasPrefix(paquete, "cmd/") || strings.HasPrefix(paquete, "internal/adapter/") {
			continue
		}
		for destino, archivo := range imports {
			if strings.HasPrefix(destino, "internal/adapter/") {
				t.Errorf("%s importa %s (en %s): solo la raiz de composicion puede elegir implementaciones",
					paquete, destino, archivo)
			}
		}
	}
}

// TestLaPruebaMiraAlgo evita el peor final posible para las de arriba: que un
// cambio en las rutas las deje recorriendo un arbol vacio y pasando siempre.
func TestLaPruebaMiraAlgo(t *testing.T) {
	grafo := importsDe(t)
	for _, obligatorio := range []string{
		"internal/domain", "internal/app", "internal/adapter/web", "cmd/mcvps",
	} {
		if _, ok := grafo[obligatorio]; !ok {
			t.Errorf("no se encontro el paquete %s: la prueba no esta comprobando nada",
				obligatorio)
		}
	}
}
