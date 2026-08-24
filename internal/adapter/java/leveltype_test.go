package java

import (
	"testing"

	"github.com/Excelion29/mc-config/internal/domain"
)

// TestLevelTypeSoloLosDeJava: un tipo que Java no entiende no debe escribirse.
//
// Si se escribiera, el servidor lo ignora EN SILENCIO y genera un mundo
// normal. Quien pidio "heredado" -que es de Bedrock- creeria tenerlo.
func TestLevelType(t *testing.T) {
	casos := map[domain.LevelType]string{
		domain.LevelNormal:      "minecraft:normal",
		domain.LevelFlat:        "minecraft:flat",
		domain.LevelLargeBiomes: "minecraft:large_biomes",
		domain.LevelAmplified:   "minecraft:amplified",
		// De Bedrock: aqui no existe, asi que no se escribe la clave.
		domain.LevelLegacy: "",
		"inventado":        "",
	}
	for entrada, quiero := range casos {
		if got := levelType(entrada); got != quiero {
			t.Errorf("levelType(%q) = %q, se esperaba %q", entrada, got, quiero)
		}
	}
}

// TestCadaEdicionSoloOfreceLoSuyo protege la promesa de la interfaz: lo que se
// ofrece es lo que el servidor entiende.
func TestCadaEdicionSoloOfreceLoSuyo(t *testing.T) {
	for _, tipo := range domain.LevelTypesFor(domain.EditionJava) {
		if levelType(tipo) == "" {
			t.Errorf("se ofrece %q en Java pero no se sabe traducir", tipo)
		}
		if !tipo.ValidFor(domain.EditionJava) {
			t.Errorf("%q deberia ser valido en Java", tipo)
		}
	}
	// Y al reves: lo de Bedrock no se cuela en Java.
	if domain.LevelLegacy.ValidFor(domain.EditionJava) {
		t.Error("el tipo heredado es de Bedrock y no deberia valer en Java")
	}
	if domain.LevelAmplified.ValidFor(domain.EditionBedrock) {
		t.Error("el amplificado es de Java y no deberia valer en Bedrock")
	}
}
