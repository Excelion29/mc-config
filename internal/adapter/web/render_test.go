package web

import "testing"

// TestPlantillasCompilan comprueba que todas las plantillas embebidas compilan.
//
// El renderer ya revienta al arrancar si alguna esta mal, pero enterarse en el
// arranque significa enterarse en el servidor, con el despliegue a medias. Un
// {{end}} de mas es un error que no detecta ni "go build" ni "go vet": el HTML
// es opaco para el compilador. Esta prueba lo mueve al pipeline, antes de que
// nada salga de la maquina.
func TestPlantillasCompilan(t *testing.T) {
	r, err := newRenderer(assets)
	if err != nil {
		t.Fatalf("las plantillas no compilan: %v", err)
	}

	// Cada pagina que el codigo pide por nombre tiene que existir de verdad.
	// Renombrar un archivo y olvidar una referencia da un 500 en produccion.
	for _, page := range []string{
		"home.html", "login.html", "error.html", "audit.html",
		"maps.html", "instances.html", "players.html",
		"users.html", "roles.html",
	} {
		if _, ok := r.pages[page]; !ok {
			t.Errorf("falta la plantilla %s", page)
		}
	}
}
