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
		p := &Pack{URL: url}
		if got := p.Automatico(); got != esperado {
			t.Errorf("Automatico(%q) = %v, se esperaba %v", url, got, esperado)
		}
	}

	if (*Pack)(nil).Automatico() {
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
		if got := PackURLValida(url); got != esperado {
			t.Errorf("PackURLValida(%q) = %v, se esperaba %v", url, got, esperado)
		}
	}
}
