package app

import (
	"context"
	"testing"

	"github.com/Excelion29/mc-config/internal/domain"
)

// TestRefrescarReleeTodoLoQueSeEscribe protege un fallo que ya cayo dos veces.
//
// WriteConfig reescribe server.properties ENTERO. Lo que no se refresque antes
// se escribe con el valor cero y DESAPARECE del archivo, sin error y sin log.
//
// Paso con el modo de autenticacion -propagar la lista de permitidos volvia a
// poner online-mode=true- y despues con el paquete de texturas -dar de alta a un
// jugador borraba el resource-pack-. Las dos veces por lo mismo: un ajuste nuevo
// que se refrescaba en un camino y no en el otro.
func TestRefrescarReleeTodoLoQueSeEscribe(t *testing.T) {
	i := instanciasDePrueba(&tiendaFalsa{puestos: map[string]bool{}})
	i.authMode = func(context.Context) domain.AuthMode { return domain.AuthOffline }
	i.rulesOf = func(context.Context, int64) (domain.Rules, error) {
		return domain.Rules{Gamemode: "creative", MaxPlayers: 20}, nil
	}
	i.packOf = func(context.Context, int64) (domain.PackRef, error) {
		return domain.PackRef{URL: "https://ejemplo.com/t.zip", SHA1: "abc", Required: true}, nil
	}

	// Una instancia recien leida de la base: el modo, las reglas y el paquete
	// NO son columnas suyas, asi que llegan vacios. Es el estado real, y es
	// donde estaba el fallo.
	inst := &domain.Instance{Name: "servidor", Slug: "servidor", WorldID: 7}
	i.refrescar(context.Background(), inst)

	if !inst.Auth.SinConexion() {
		t.Error("el modo no se releyo; server.properties saldria con online-mode=true")
	}
	if inst.Rules.Gamemode != "creative" || inst.Rules.MaxPlayers != 20 {
		t.Errorf("las reglas no se releyeron: %+v", inst.Rules)
	}
	if inst.Pack.URL == "" || inst.Pack.SHA1 == "" || !inst.Pack.Required {
		t.Errorf("el paquete no se releyo; las texturas desapareceria del archivo: %+v", inst.Pack)
	}
}

// TestRefrescarNoRompeSiFallaUnaFuente: no impedir arrancar un servidor porque
// no se pudo consultar un ajuste. Se sigue con lo que traiga la instancia.
func TestRefrescarNoRompeSiFallaUnaFuente(t *testing.T) {
	i := instanciasDePrueba(&tiendaFalsa{puestos: map[string]bool{}})
	i.rulesOf = func(context.Context, int64) (domain.Rules, error) {
		return domain.Rules{}, context.DeadlineExceeded
	}
	i.packOf = func(context.Context, int64) (domain.PackRef, error) {
		return domain.PackRef{}, context.DeadlineExceeded
	}

	inst := &domain.Instance{
		Name: "servidor", Slug: "servidor", WorldID: 7,
		Rules: domain.Rules{Gamemode: "survival", MaxPlayers: 12},
	}
	i.refrescar(context.Background(), inst)

	if inst.Rules.Gamemode != "survival" || inst.Rules.MaxPlayers != 12 {
		t.Errorf("se perdieron las reglas que ya tenia: %+v", inst.Rules)
	}
	// Y el modo, que no depende del mundo, tiene que quedar puesto igualmente.
	if !inst.Auth.Valid() {
		t.Error("el modo deberia quedar en un valor valido aunque fallen los demas")
	}
}
