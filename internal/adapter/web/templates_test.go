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
		"instances.html", "worlds.html", "access.html",
		"users.html", "roles.html", "resources.html",
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

// TestLosBotonesDeCabeceraSeAlinean protege la alineacion de las cabeceras.
//
// El boton de accion tiene que ir dentro de .cabecera-acciones, que es lo que
// lo empuja al borde derecho. Suelto queda flotando a media pagina, con el
// texto apretado a un lado y un hueco muerto al otro.
//
// Paso justo eso: se arreglo en una pantalla y las otras cuatro se quedaron
// atras, porque nada obligaba a que fueran iguales.
func TestLosBotonesDeCabeceraSeAlinean(t *testing.T) {
	paginas, err := fs.Glob(assets, "templates/*.html")
	if err != nil {
		t.Fatal(err)
	}

	revisadas, conAccion := 0, 0
	for _, ruta := range paginas {
		datos, err := fs.ReadFile(assets, ruta)
		if err != nil {
			t.Fatal(err)
		}
		html := string(datos)
		if !strings.Contains(html, `class="cabecera-pagina"`) {
			continue
		}
		revisadas++

		// Solo se exige a las cabeceras que TIENEN una accion. Una pantalla
		// puede no tener ninguna -la de acceso no la tiene, porque sus botones
		// van junto a lo que explican- y forzarla a inventarse una para pasar
		// la prueba seria dejar que la prueba mande sobre el diseno.
		cab := bloqueCabecera(html)
		if !strings.Contains(cab, "<button") && !strings.Contains(cab, "boton") {
			continue
		}
		conAccion++

		if !strings.Contains(cab, `class="cabecera-acciones"`) {
			t.Errorf("%s: tiene cabecera pero el boton no esta en .cabecera-acciones", ruta)
		}
	}

	if conAccion == 0 {
		t.Error("ninguna cabecera tiene accion; la prueba no comprueba nada")
	}

	if revisadas == 0 {
		t.Error("no se reviso ninguna cabecera; la prueba no comprueba nada")
	}
}

// bloqueCabecera devuelve el <div class="cabecera-pagina"> completo.
//
// Se cuentan las aperturas y los cierres en vez de buscar el primer </div>,
// porque la cabecera lleva un div dentro para el titulo y el subtitulo: con el
// primer cierre nos quedariamos con la mitad y el boton, que va al final, se
// escaparia siempre.
func bloqueCabecera(html string) string {
	i := strings.Index(html, `class="cabecera-pagina"`)
	if i < 0 {
		return ""
	}
	// Retrocede hasta el "<div" que abre.
	inicio := strings.LastIndex(html[:i], "<div")
	if inicio < 0 {
		return ""
	}

	hondura := 0
	for j := inicio; j < len(html); j++ {
		switch {
		case strings.HasPrefix(html[j:], "<div"):
			hondura++
		case strings.HasPrefix(html[j:], "</div>"):
			hondura--
			if hondura == 0 {
				return html[inicio : j+len("</div>")]
			}
		}
	}
	return html[inicio:]
}

// TestLasAccionesDeUnaTablaSeAlinean protege un desajuste que solo se ve mirando.
//
// La cabecera "Acciones" va alineada a la derecha con .derecha. Si las celdas no
// llevan .acciones-fila -que es lo que las empuja al mismo borde- los iconos se
// quedan a media tabla y el titulo apunta a un hueco vacio.
//
// Paso justo eso en la biblioteca de recursos: se uso .acciones, que agrupa pero
// no alinea. No da error, no rompe nada, y queda torcido.
func TestLasAccionesDeUnaTablaSeAlinean(t *testing.T) {
	paginas, err := fs.Glob(assets, "templates/*.html")
	if err != nil {
		t.Fatal(err)
	}

	revisadas := 0
	for _, ruta := range paginas {
		datos, err := fs.ReadFile(assets, ruta)
		if err != nil {
			t.Fatal(err)
		}
		html := string(datos)

		if !strings.Contains(html, `class="derecha">Acciones`) {
			continue
		}
		revisadas++

		if !strings.Contains(html, `class="acciones-fila"`) {
			t.Errorf("%s: la cabecera de acciones va a la derecha pero las celdas "+
				"no usan .acciones-fila, asi que los iconos no llegan al borde", ruta)
		}
	}

	if revisadas == 0 {
		t.Error("ninguna tabla tiene cabecera de acciones; la prueba no comprueba nada")
	}
}
