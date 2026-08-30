package web

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Excelion29/mc-config/internal/app"
	"github.com/Excelion29/mc-config/internal/domain"
)

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
		"worlds.html", "instances.html",
		"users.html", "roles.html", "access.html", "resources.html",
	} {
		if _, ok := r.pages[page]; !ok {
			t.Errorf("falta la plantilla %s", page)
		}
	}
}

// TestLaPaginaDeAccesoSePinta ejecuta la plantilla en sus tres situaciones.
//
// Que compile no basta: una plantilla solo falla al EJECUTARSE, y un campo mal
// escrito o un metodo que no existe da un 500 en blanco. Las tres situaciones
// son estados de verdad -sin servidor, con plugins por poner, y ya abierto- y
// cada una pinta una parte distinta del archivo, asi que ninguna rama se queda
// sin ejecutar nunca.
func TestLaPaginaDeAccesoSePinta(t *testing.T) {
	r, err := newRenderer(assets)
	if err != nil {
		t.Fatal(err)
	}

	plugins := []app.Plugin{{Name: "AuthMe", File: "AuthMe.jar", Why: "pide contrasena"}}

	casos := []struct {
		nombre string
		estado app.Estado
		espera string
	}{
		{
			nombre: "sin servidor de java",
			estado: app.Estado{Mode: domain.AuthOnline},
			espera: "crea uno",
		},
		{
			nombre: "faltan plugins",
			estado: app.Estado{
				Mode:       domain.AuthOnline,
				Instancias: []string{"LosDelSotano", "Creativo"},
				Requeridos: plugins,
				Faltan:     plugins,
				Filas:      []app.PluginRow{{Plugin: plugins[0], Puesto: false}},
			},
			espera: "Instalar complementos",
		},
		{
			nombre: "ya abierto",
			estado: app.Estado{
				Mode:       domain.AuthOffline,
				Instancias: []string{"LosDelSotano", "Creativo"},
				Requeridos: plugins,
				Filas:      []app.PluginRow{{Plugin: plugins[0], Puesto: true}},
			},
			espera: "Volver a solo cuentas compradas",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.render(w, 200, "access.html", accessPageData{
				PageData: PageData{Title: "Acceso", User: &domain.User{}},
				// Puede: quien opera el servidor. Sin esto no se pinta la mitad
				// del modo, que es justo lo que esta prueba mira.
				Puede:  true,
				Estado: c.estado,
			})

			if w.Code != 200 {
				t.Fatalf("devolvio %d", w.Code)
			}
			if !strings.Contains(w.Body.String(), c.espera) {
				t.Errorf("no aparece %q en la pagina", c.espera)
			}
		})
	}
}
