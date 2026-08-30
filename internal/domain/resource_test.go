package domain

import "testing"

// TestQueEnlacesSePuedenAplicarSolos fija la regla que decide todo lo demas.
//
// Java descarga el paquete pidiendo la URL tal cual. Si esa URL devuelve una
// pagina web en vez del archivo, el cliente recibe HTML esperando un zip: no
// carga las texturas y no dice por que.
//
// La mayoria de sitios de mapas dan una pagina de descarga -MediaFire, Drive-
// asi que este caso es el normal, no la excepcion. Por eso el panel los guarda
// igual y solo se niega a aplicarlos solo.
func TestQueEnlacesSePuedenAplicarSolos(t *testing.T) {
	casos := map[string]bool{
		"https://ejemplo.com/texturas.zip":         true,
		"https://ejemplo.com/Texturas.ZIP":         true, // el servidor no distingue
		"https://ejemplo.com/texturas.zip?v=2":     true, // los parametros no cuentan
		"https://ejemplo.com/texturas.zip#nota":    true,
		"https://mediafire.com/file/abc/texturas":  false,
		"https://drive.google.com/file/d/abc/view": false,
		"https://ejemplo.com/descargar":            false,
		"":                                         false,
	}

	for url, esperado := range casos {
		p := &Resource{URL: url}
		if got := p.Automatico(); got != esperado {
			t.Errorf("Automatico(%q) = %v, se esperaba %v", url, got, esperado)
		}
	}

	if (*Resource)(nil).Automatico() {
		t.Error("un paquete que no existe no se aplica solo")
	}
}

// TestSoloHttps: ese archivo acaba dentro del juego de cada persona que entra.
//
// No es lo mismo que con las portadas -alli el problema era el contenido mixto
// del navegador-. Aqui lo descarga Minecraft, y por http cualquiera en el
// camino puede cambiarlo sin que se note.
func TestSoloHttps(t *testing.T) {
	casos := map[string]bool{
		"https://ejemplo.com/t.zip": true,
		"http://ejemplo.com/t.zip":  false,
		"ftp://ejemplo.com/t.zip":   false,
		"ejemplo.com/t.zip":         false,
		"":                          false,
		"   ":                       false,
	}

	for url, esperado := range casos {
		if got := RecursoURLValida(url); got != esperado {
			t.Errorf("RecursoURLValida(%q) = %v, se esperaba %v", url, got, esperado)
		}
	}
}

// TestElNombreEsUnaMascaraDelEnlace fija de donde sale lo que se lee en pantalla.
//
// El enlace es lo unico obligatorio: bautizar cada cosa que pegas es trabajo, y
// casi siempre se puede deducir. Manda lo que alguien escribio; si no, el titulo
// que el panel saco de la pagina; y si tampoco, el enlace tal cual.
//
// Nunca queda en blanco. Una fila sin nada que leer no se puede ni elegir ni
// borrar con criterio.
func TestElNombreEsUnaMascaraDelEnlace(t *testing.T) {
	casos := []struct {
		nombre   string
		recurso  Resource
		etiqueta string
		puesto   bool
	}{
		{
			nombre:   "manda el nombre puesto a mano",
			recurso:  Resource{Name: "Medievales", AutoName: "Descargar | MiSitio", URL: "https://x.com/t.zip"},
			etiqueta: "Medievales",
			puesto:   true,
		},
		{
			nombre:   "sin nombre, el titulo de la pagina",
			recurso:  Resource{AutoName: "Texturas Medievales", URL: "https://x.com/p"},
			etiqueta: "Texturas Medievales",
		},
		{
			nombre:   "sin nada, el enlace",
			recurso:  Resource{URL: "https://x.com/p"},
			etiqueta: "https://x.com/p",
		},
		{
			nombre:   "un nombre en blanco no cuenta como nombre",
			recurso:  Resource{Name: "   ", AutoName: "Titulo", URL: "https://x.com/p"},
			etiqueta: "Titulo",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := c.recurso.Etiqueta(); got != c.etiqueta {
				t.Errorf("Etiqueta() = %q, se esperaba %q", got, c.etiqueta)
			}
			if got := c.recurso.Bautizado(); got != c.puesto {
				t.Errorf("Bautizado() = %v, se esperaba %v", got, c.puesto)
			}
		})
	}
}

// TestElNombreSacadoDelEnlaceDiceAlgo protege contra los nombres de relleno.
//
// Paso en pantalla: un recurso llamado "download", porque el enlace acababa en
// /download y se tomo la ultima parte de la ruta como si fuera el nombre del
// archivo. No lo era, y ademas media internet acaba igual: dos paquetes
// distintos se habrian llamado los dos "download".
func TestElNombreSacadoDelEnlaceDiceAlgo(t *testing.T) {
	casos := map[string]string{
		// Un archivo de verdad: se usa tal cual.
		"https://ejemplo.com/texturas-medievales.zip":   "texturas-medievales.zip",
		"https://ejemplo.com/packs/v2/texturas.zip?d=1": "texturas.zip",

		// Palabras de relleno: mejor el dominio, que al menos dice de donde sale.
		"https://ejemplo.com/download":       "ejemplo.com",
		"https://www.mediafire.com/file/abc": "mediafire.com",
		"https://ejemplo.com/descargas":      "ejemplo.com",
		"https://ejemplo.com/":               "ejemplo.com",
		"https://ejemplo.com":                "ejemplo.com",
	}

	for url, esperado := range casos {
		if got := TituloDeEnlace(url); got != esperado {
			t.Errorf("TituloDeEnlace(%q) = %q, se esperaba %q", url, got, esperado)
		}
	}
}
