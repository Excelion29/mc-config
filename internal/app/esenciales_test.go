package app

import (
	"testing"

	"github.com/Excelion29/mc-config/internal/domain"
)

// TestSoloLoQueProtegeImpideAbrirElAcceso separa la seguridad de la comodidad.
//
// El panel se niega a abrir el modo sin conexion si falta AuthMe, y hace bien:
// sin el, cualquiera entra con el nombre que quiera. Pero SkinsRestorer no
// protege nada -sin el se pierden las skins- y bloquear por eso seria confundir
// las dos cosas y dejar el servidor cerrado por un motivo estetico.
func TestSoloLoQueProtegeImpideAbrirElAcceso(t *testing.T) {
	protege := Plugin{ID: "authme", Name: "AuthMe", File: "AuthMe.jar", Esencial: true}
	mejora := Plugin{ID: "skins", Name: "SkinsRestorer", File: "Skins.jar"}

	casos := []struct {
		nombre string
		estado Estado
		listo  bool
	}{
		{
			nombre: "todo puesto",
			estado: Estado{Instancias: []string{"servidor"}},
			listo:  true,
		},
		{
			nombre: "falta el que protege",
			estado: Estado{
				Instancias:       []string{"servidor"},
				Faltan:           []Plugin{protege},
				FaltanEsenciales: []Plugin{protege},
			},
			listo: false,
		},
		{
			nombre: "falta solo el que mejora",
			estado: Estado{
				Instancias: []string{"servidor"},
				Faltan:     []Plugin{mejora},
			},
			// Se puede abrir: quedarse sin skins es molesto, no peligroso.
			listo: true,
		},
		{
			nombre: "sin ningun servidor de Java no hay donde abrirlo",
			estado: Estado{},
			listo:  false,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := c.estado.Listo(); got != c.listo {
				t.Errorf("Listo() = %v, se esperaba %v", got, c.listo)
			}
		})
	}
}

// TestLosDosComplementosSeInstalanIgual: que uno no bloquee no significa que no
// se ponga. La diferencia es solo si su ausencia impide abrir el acceso.
func TestLosDosComplementosSeInstalanIgual(t *testing.T) {
	tienda := &tiendaFalsa{puestos: map[string]bool{}}
	i := instanciasDePrueba(tienda)
	i.flavors[domain.EditionJava] = saborConDos{}

	inst := &domain.Instance{
		Name: "servidor", Slug: "servidor",
		Edition: domain.EditionJava, Auth: domain.AuthOffline,
	}
	if err := i.asegurarPlugins(nil, inst); err != nil {
		t.Fatal(err)
	}

	if len(tienda.instalado) != 2 {
		t.Errorf("se instalaron %d complementos, se esperaban los 2", len(tienda.instalado))
	}
}

type saborConDos struct{ saborFalso }

func (saborConDos) PluginsFor(mode domain.AuthMode) []Plugin {
	if !mode.SinConexion() {
		return nil
	}
	return []Plugin{
		{ID: "authme", Name: "AuthMe", File: "AuthMe.jar", Esencial: true},
		{ID: "skins", Name: "SkinsRestorer", File: "Skins.jar"},
	}
}
