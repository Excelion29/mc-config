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

	plugins := []app.Plugin{{
		ID: "authme", Name: "AuthMe", File: "AuthMe-6.0.0-Paper.jar",
		URL:  "https://github.com/AuthMe/AuthMeReloaded/releases/download/6.0.0/AuthMe-6.0.0-Paper.jar",
		Docs: "https://github.com/AuthMe/AuthMeReloaded",
		Why:  "pide contrasena", DeFabrica: true,
	}}

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

// TestLaFichaDelComplementoEnsenaSuVersion protege lo que hace falta para poder
// decidir si subirlo.
//
// Sin ver que archivo lleva puesto ni poder abrir su documentacion, cambiar de
// version seria pegar un enlace a ciegas. Y sin el campo, habria que desplegar
// para subir un plugin, que es la forma de no subirlo nunca.
func TestLaFichaDelComplementoEnsenaSuVersion(t *testing.T) {
	r, err := newRenderer(assets)
	if err != nil {
		t.Fatal(err)
	}

	fabrica := app.Plugin{
		ID: "authme", Name: "AuthMe", File: "AuthMe-6.0.0-Paper.jar",
		URL:  "https://github.com/AuthMe/AuthMeReloaded/releases/download/6.0.0/AuthMe-6.0.0-Paper.jar",
		Docs: "https://github.com/AuthMe/AuthMeReloaded",
		Why:  "pide contrasena", DeFabrica: true,
	}
	propia := fabrica
	propia.File, propia.DeFabrica = "AuthMe-6.1.0-Paper.jar", false

	casos := []struct {
		nombre  string
		plugin  app.Plugin
		esperar []string
		evitar  []string
	}{
		{
			nombre:  "con la version de fabrica",
			plugin:  fabrica,
			esperar: []string{"AuthMe-6.0.0-Paper.jar", "github.com/AuthMe/AuthMeReloaded", "/access/plugin-version"},
			evitar:  []string{"version propia"},
		},
		{
			nombre: "con una version elegida a mano",
			plugin: propia,
			// Que la version no es la de fabrica tiene que verse: es lo que
			// explica por que este panel no se comporta como otro igual.
			esperar: []string{"AuthMe-6.1.0-Paper.jar", "version propia"},
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.render(w, 200, "access.html", accessPageData{
				PageData: PageData{Title: "Acceso", User: &domain.User{}},
				Puede:    true,
				Estado: app.Estado{
					Mode:       domain.AuthOnline,
					Instancias: []string{"LosDelSotano"},
					Requeridos: []app.Plugin{c.plugin},
					Filas:      []app.PluginRow{{Plugin: c.plugin, Puesto: true}},
				},
			})

			cuerpo := w.Body.String()
			for _, quiero := range c.esperar {
				if !strings.Contains(cuerpo, quiero) {
					t.Errorf("falta %q en la pantalla", quiero)
				}
			}
			for _, sobra := range c.evitar {
				if strings.Contains(cuerpo, sobra) {
					t.Errorf("no deberia aparecer %q", sobra)
				}
			}
		})
	}
}
