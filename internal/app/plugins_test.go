package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/Excelion29/mc-config/internal/domain"
)

// tiendaFalsa recuerda lo que le pidieron instalar y finge lo que ya hay.
type tiendaFalsa struct {
	puestos   map[string]bool
	instalado []Plugin
	falla     error
}

func (t *tiendaFalsa) Install(_ context.Context, _ string, plugins []Plugin) error {
	if t.falla != nil {
		return t.falla
	}
	t.instalado = append(t.instalado, plugins...)
	for _, p := range plugins {
		t.puestos[p.File] = true
	}
	return nil
}

func (t *tiendaFalsa) Installed(_ string, plugins []Plugin) []Plugin {
	var out []Plugin
	for _, p := range plugins {
		if t.puestos[p.File] {
			out = append(out, p)
		}
	}
	return out
}

// saborFalso es una edicion que pide un plugin en modo sin conexion.
type saborFalso struct{ ServerFlavor }

func (saborFalso) Edition() domain.Edition { return domain.EditionJava }
func (saborFalso) DefaultPort() int        { return 25565 }

// Spec imita lo que hace el sabor de Java: el modo sale de inst.Auth.
func (saborFalso) Spec(inst *domain.Instance, dir string) ContainerSpec {
	modo := "TRUE"
	if inst.Auth.SinConexion() {
		modo = "FALSE"
	}
	return ContainerSpec{
		Name:    "mcvps-" + inst.Slug,
		Image:   "imagen",
		Env:     map[string]string{"ONLINE_MODE": modo},
		DataDir: dir,
	}
}

func (saborFalso) PluginsFor(mode domain.AuthMode) []Plugin {
	if !mode.SinConexion() {
		return nil
	}
	return []Plugin{{Name: "AuthMe", File: "AuthMe.jar"}}
}

func instanciasDePrueba(tienda PluginStore) *Instances {
	return &Instances{
		flavors:  map[domain.Edition]ServerFlavor{domain.EditionJava: saborFalso{}},
		plugins:  tienda,
		dataRoot: "/tmp",
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// TestArrancarInstalaLosComplementosQueFaltan protege la red de seguridad del
// modo sin conexion.
//
// El modo es GLOBAL: se aplica a cualquier servidor que arranque, se creara
// antes o despues de activarlo. Si los complementos solo se instalaran desde la
// pantalla de Acceso, un servidor creado despues arrancaria abierto y sin
// AuthMe, y eso es cualquiera entrando con el nombre que quiera.
//
// Y no avisaria de nada: el servidor arranca perfectamente. Por eso se
// comprueba en cada arranque y no una sola vez.
func TestArrancarInstalaLosComplementosQueFaltan(t *testing.T) {
	casos := []struct {
		nombre  string
		modo    domain.AuthMode
		yaHay   map[string]bool
		instala int
	}{
		{
			nombre:  "modo normal no pide nada",
			modo:    domain.AuthOnline,
			yaHay:   map[string]bool{},
			instala: 0,
		},
		{
			nombre:  "sin conexion instala lo que falta",
			modo:    domain.AuthOffline,
			yaHay:   map[string]bool{},
			instala: 1,
		},
		{
			nombre:  "no los vuelve a bajar si ya estan",
			modo:    domain.AuthOffline,
			yaHay:   map[string]bool{"AuthMe.jar": true},
			instala: 0,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			tienda := &tiendaFalsa{puestos: c.yaHay}
			inst := &domain.Instance{
				Name: "servidor", Slug: "servidor",
				Edition: domain.EditionJava, Auth: c.modo,
			}

			if err := instanciasDePrueba(tienda).asegurarPlugins(context.Background(), inst); err != nil {
				t.Fatalf("no deberia fallar: %v", err)
			}
			if len(tienda.instalado) != c.instala {
				t.Errorf("instalo %d complementos, se esperaban %d",
					len(tienda.instalado), c.instala)
			}
		})
	}
}

// TestNoArrancaSiNoPuedeInstalarLosComplementos comprueba que se niega.
//
// Arrancar igual seria el peor de los dos fallos: un servidor que funciona, no
// da ningun error y esta abierto de par en par. Mas vale un arranque que falla
// y se explica.
func TestNoArrancaSiNoPuedeInstalarLosComplementos(t *testing.T) {
	quiebra := errors.New("sin red")
	tienda := &tiendaFalsa{puestos: map[string]bool{}, falla: quiebra}

	inst := &domain.Instance{
		Name: "servidor", Slug: "servidor",
		Edition: domain.EditionJava, Auth: domain.AuthOffline,
	}

	err := instanciasDePrueba(tienda).asegurarPlugins(context.Background(), inst)
	if !errors.Is(err, quiebra) {
		t.Fatalf("deberia negarse a arrancar, devolvio %v", err)
	}
}

// TestSinTiendaDeComplementosNoSeArrancaAbierto cubre el panel mal configurado.
//
// La tienda puede ser nil -sin MCVPS_PLUGINS_PATH el panel funciona igual- y
// entonces NO hay forma de instalar nada. Lo que no puede pasar es que esa
// ausencia se lea como "no hacia falta ninguno" y el servidor arranque abierto.
func TestSinTiendaDeComplementosNoSeArrancaAbierto(t *testing.T) {
	i := instanciasDePrueba(nil)
	i.plugins = nil

	inst := &domain.Instance{
		Name: "servidor", Slug: "servidor",
		Edition: domain.EditionJava, Auth: domain.AuthOffline,
	}

	if err := i.asegurarPlugins(context.Background(), inst); !errors.Is(err, domain.ErrPluginsUnavailable) {
		t.Fatalf("deberia negarse por falta de tienda, devolvio %v", err)
	}
}

// TestElModoLlegaAlContenedor protege el fallo que dejo F6 sin efecto.
//
// El sabor construye ONLINE_MODE a partir de inst.Auth. Antes el modo se leia
// DESPUES de pedirle la definicion, sobre un campo del spec que no leia nadie,
// y al crear una instancia inst.Auth venia vacio. Resultado: el contenedor
// exigia cuenta comprada SIEMPRE, mientras el panel decia que el acceso estaba
// abierto.
//
// No dio ningun error en ningun sitio. El servidor arrancaba perfecto; lo unico
// que se veia era un amigo que no podia entrar.
func TestElModoLlegaAlContenedor(t *testing.T) {
	for _, modo := range []domain.AuthMode{domain.AuthOnline, domain.AuthOffline} {
		i := instanciasDePrueba(&tiendaFalsa{puestos: map[string]bool{}})
		i.authMode = func(context.Context) domain.AuthMode { return modo }

		// Sin tocar inst.Auth a proposito: es como llega una instancia recien
		// creada, y es donde estaba el fallo.
		inst := &domain.Instance{Name: "servidor", Slug: "servidor", Edition: domain.EditionJava}
		spec := i.specDe(context.Background(), saborFalso{}, inst, "/tmp/servidor")

		esperado := "TRUE"
		if modo.SinConexion() {
			esperado = "FALSE"
		}
		if spec.Env["ONLINE_MODE"] != esperado {
			t.Errorf("modo %s: ONLINE_MODE = %q, se esperaba %q",
				modo, spec.Env["ONLINE_MODE"], esperado)
		}
	}
}

// TestLaHuellaCambiaConElModo es lo que obliga a rehacer el contenedor.
//
// Las variables de entorno se fijan al crearlo: si la huella no cambiara,
// cambiar el modo no llegaria nunca a un servidor ya creado, por muchas veces
// que se reinicie.
func TestLaHuellaCambiaConElModo(t *testing.T) {
	huella := func(modo domain.AuthMode) string {
		i := instanciasDePrueba(&tiendaFalsa{puestos: map[string]bool{}})
		i.authMode = func(context.Context) domain.AuthMode { return modo }
		inst := &domain.Instance{Name: "servidor", Slug: "servidor", Edition: domain.EditionJava}
		return i.specDe(context.Background(), saborFalso{}, inst, "/tmp/servidor").Huella()
	}

	if huella(domain.AuthOnline) == huella(domain.AuthOffline) {
		t.Error("la huella deberia cambiar con el modo; si no, el contenedor viejo se reutiliza")
	}
	if huella(domain.AuthOffline) != huella(domain.AuthOffline) {
		t.Error("la huella tiene que ser estable, o el contenedor se rehace en cada arranque")
	}
}
