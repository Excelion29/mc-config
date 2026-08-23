package web

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

var (
	botonModal = regexp.MustCompile(`class="boton"\s+href="#([a-z0-9-]+)"`)
	enlaceHash = regexp.MustCompile(`<a[^>]*href="#([a-z0-9-]+)"`)
)

// TestBotonesAbrenUnModalQueExiste comprueba que todo enlace a "#algo" apunta a
// un id que de verdad esta en la misma plantilla.
//
// Los dialogos se abren con :target, asi que un destino equivocado no falla:
// simplemente no pasa nada al pulsar. No lo detecta el compilador, ni go vet,
// ni la prueba que compila las plantillas, y en pantalla se ve idéntico a un
// boton que funciona. Solo se nota pulsandolo.
func TestBotonesAbrenUnModalQueExiste(t *testing.T) {
	paginas, err := fs.Glob(assets, "templates/*.html")
	if err != nil {
		t.Fatal(err)
	}

	for _, ruta := range paginas {
		datos, err := fs.ReadFile(assets, ruta)
		if err != nil {
			t.Fatal(err)
		}
		html := string(datos)

		for _, m := range enlaceHash.FindAllStringSubmatch(html, -1) {
			destino := m[1]
			if !strings.Contains(html, `id="`+destino+`"`) {
				t.Errorf("%s: el enlace a #%s no apunta a ningun id de esa plantilla",
					ruta, destino)
			}
		}
	}
}

// TestCadaPantallaTieneSuAccion protege el patron: las pantallas de listado se
// crean con un boton en la cabecera que abre un dialogo, no con pestanas.
//
// Sin esto, perder el boton en una reescritura deja la pantalla mirandose a si
// misma: la lista se ve, pero no hay forma de anadir nada.
func TestCadaPantallaTieneSuAccion(t *testing.T) {
	for _, pagina := range []string{
		"instances.html", "worlds.html", "players.html",
		"users.html", "roles.html",
	} {
		datos, err := fs.ReadFile(assets, "templates/"+pagina)
		if err != nil {
			t.Fatal(err)
		}
		html := string(datos)

		if !strings.Contains(html, `class="cabecera-pagina"`) {
			t.Errorf("%s: no tiene cabecera con accion", pagina)
			continue
		}
		if !botonModal.MatchString(html) {
			t.Errorf("%s: la cabecera no tiene ningun boton que abra un dialogo", pagina)
		}
		// Las pestanas se retiraron: dos formas de hacer lo mismo acaban
		// mezclandose.
		if strings.Contains(html, `class="tabs"`) {
			t.Errorf("%s: quedan pestanas; el patron es lista + boton + dialogo", pagina)
		}
	}
}

var claseEstatica = regexp.MustCompile(`class="([^"{}]*)"`)

// TestCadaClaseTieneEstilo comprueba que toda clase escrita en una plantilla
// existe en alguna hoja de estilos.
//
// Una clase mal escrita no rompe nada: el navegador la ignora y la pagina sale
// sin ese estilo, a veces de forma casi imperceptible. Al repartir el CSS en
// capas -tokens, atomos, moleculas, organismos, layout- el riesgo de perder
// una regla por el camino sube, y esto lo cierra.
//
// Solo mira las clases literales: las que se componen en la plantilla, como
// "estado-{{.State}}", no se pueden comprobar asi y van declaradas a mano
// junto a su atomo.
func TestCadaClaseTieneEstilo(t *testing.T) {
	hojas, err := fs.Glob(assets, "static/css/*.css")
	if err != nil || len(hojas) == 0 {
		t.Fatalf("no se encontraron hojas de estilo: %v", err)
	}

	var css strings.Builder
	for _, h := range hojas {
		datos, err := fs.ReadFile(assets, h)
		if err != nil {
			t.Fatal(err)
		}
		css.Write(datos)
	}
	estilos := css.String()

	paginas, err := fs.Glob(assets, "templates/*.html")
	if err != nil {
		t.Fatal(err)
	}

	for _, ruta := range paginas {
		datos, err := fs.ReadFile(assets, ruta)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range claseEstatica.FindAllStringSubmatch(string(datos), -1) {
			for _, clase := range strings.Fields(m[1]) {
				if !strings.Contains(estilos, "."+clase) {
					t.Errorf("%s: la clase %q no tiene ninguna regla", ruta, clase)
				}
			}
		}
	}
}

var accionIcono = regexp.MustCompile(`(?s)<(?:button|a)\b[^>]*class="accion[^"]*"[^>]*>`)

// TestLasAccionesDeIconoTienenNombre exige aria-label y title en todo boton que
// solo muestra un icono.
//
// Al quitar el texto, el dibujo es lo unico que queda: sin aria-label un lector
// de pantalla anuncia "boton" a secas, y sin title quien ve la pantalla tiene
// que adivinar que hace una papelera junto a un candado. La diferencia entre
// "bloquear" y "borrar" no puede quedar en manos de la interpretacion.
//
// Va en una prueba y no en una nota porque es la clase de detalle que se cae
// al anadir la sexta accion con prisa.
func TestLasAccionesDeIconoTienenNombre(t *testing.T) {
	paginas, err := fs.Glob(assets, "templates/*.html")
	if err != nil {
		t.Fatal(err)
	}

	total := 0
	for _, ruta := range paginas {
		datos, err := fs.ReadFile(assets, ruta)
		if err != nil {
			t.Fatal(err)
		}
		for _, etiqueta := range accionIcono.FindAllString(string(datos), -1) {
			total++
			if !strings.Contains(etiqueta, "aria-label=") {
				t.Errorf("%s: accion de icono sin aria-label: %s", ruta, resumen(etiqueta))
			}
			if !strings.Contains(etiqueta, "title=") {
				t.Errorf("%s: accion de icono sin title: %s", ruta, resumen(etiqueta))
			}
		}
	}

	if total == 0 {
		t.Error("no se encontro ninguna accion de icono; la prueba no esta comprobando nada")
	}
}

func resumen(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 90 {
		return s[:90] + "..."
	}
	return s
}
