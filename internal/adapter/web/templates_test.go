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
		"instances.html", "maps.html", "players.html",
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
