package app

import (
	"testing"

	"github.com/Excelion29/mc-config/internal/domain"
)

// TestLosOperadoresUsanLaIdentidadDeSuEdicion protege un fallo que dejaba a
// alguien como operador en el panel y sin poder usar comandos en el juego.
//
// OpsFrom estaba escrito solo para Bedrock: metia el XUID de Xbox en ops.json de
// un servidor de JAVA. El servidor no sabe que es ese numero, asi que ignoraba
// la entrada. Y no daba error: el archivo se escribia perfectamente.
//
// La lista de permitidos ya sabia distinguir las dos ediciones; los operadores
// iban por otro camino y se quedaron atras.
func TestLosOperadoresUsanLaIdentidadDeSuEdicion(t *testing.T) {
	jugadores := []domain.Player{
		{
			Gamertag: "Wronkow29", XUID: "2535000000000000",
			JavaName: "Areku29", JavaUUID: "11111111-2222-3333-4444-555555555555",
			IsOp: true,
		},
		{Gamertag: "OtroAmigo", XUID: "2535111111111111"}, // no es operador
	}

	t.Run("en Java manda el UUID, no el XUID", func(t *testing.T) {
		ops := OpsPara(domain.EditionJava, domain.AuthOnline, jugadores)

		if len(ops) != 1 {
			t.Fatalf("deberia haber un solo operador, hay %d", len(ops))
		}
		if ops[0].ID == "2535000000000000" {
			t.Error("se colo el XUID de Xbox en un servidor de Java")
		}
		if ops[0].ID != "11111111-2222-3333-4444-555555555555" {
			t.Errorf("se esperaba el UUID de Mojang, dio %q", ops[0].ID)
		}
		if ops[0].Name != "Areku29" {
			t.Errorf("en Java el nombre es el de Java, dio %q", ops[0].Name)
		}
	})

	t.Run("con el acceso abierto valen sus dos identidades", func(t *testing.T) {
		ops := OpsPara(domain.EditionJava, domain.AuthOffline, jugadores)

		// Cual de las dos llega lo decide el modo, y por eso van las dos: la
		// misma razon que en la lista de permitidos.
		if len(ops) != 2 {
			t.Fatalf("se esperaban las dos identidades, hay %d", len(ops))
		}
		if ops[0].ID != domain.OfflineUUID("Areku29") {
			t.Errorf("falta el UUID calculado del nombre, dio %q", ops[0].ID)
		}
	})

	t.Run("en Bedrock sigue mandando el XUID", func(t *testing.T) {
		ops := OpsPara(domain.EditionBedrock, domain.AuthOnline, jugadores)

		if len(ops) != 1 || ops[0].ID != "2535000000000000" {
			t.Errorf("Bedrock identifica por XUID: %+v", ops)
		}
		if ops[0].Name != "Wronkow29" {
			t.Errorf("en Bedrock el nombre es el gamertag, dio %q", ops[0].Name)
		}
	})

	t.Run("quien no es operador no entra en la lista", func(t *testing.T) {
		for _, e := range []domain.Edition{domain.EditionJava, domain.EditionBedrock} {
			for _, ref := range OpsPara(e, domain.AuthOnline, jugadores) {
				if ref.Name == "OtroAmigo" {
					t.Errorf("%s: se colo alguien que no es operador", e)
				}
			}
		}
	})
}

// TestSePuedeSerOperadorEnJavaSinHaberEntrado quita una espera que no tocaba.
//
// En Bedrock hay que esperar a que la persona entre: su identidad es el XUID y
// no se conoce hasta entonces (D-14). En Java se sabe desde el alta, asi que
// exigir lo mismo era arrastrar la limitacion de una edicion a la otra.
func TestSePuedeSerOperadorEnJavaSinHaberEntrado(t *testing.T) {
	soloJava := &domain.Player{JavaName: "Areku29"}
	if !soloJava.PuedeSerOp() {
		t.Error("en Java no hace falta esperar a que entre para hacerle operador")
	}

	soloBedrock := &domain.Player{Gamertag: "Wronkow29"}
	if soloBedrock.PuedeSerOp() {
		t.Error("en Bedrock si hay que esperar: sin XUID no se sabe quien es")
	}
}
