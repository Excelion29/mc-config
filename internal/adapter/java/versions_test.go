package java

import (
	"strings"
	"testing"
)

// TestLeerVersionesConservaElOrden es lo importante de todo esto: la respuesta
// agrupa por familia, ya viene ordenada, y decodificarla a un map de Go
// perderia ese orden porque recorrer un map da un orden distinto cada vez.
//
// La forma sale de la respuesta REAL de fill.papermc.io, no de la que yo
// recordaba: la v2 esta retirada y devuelve 410.
func TestLeerVersionesConservaElOrden(t *testing.T) {
	cuerpo := `{"project":{"id":"paper"},"versions":{
		"26.2":["26.2","26.2-rc-2"],
		"26.1":["26.1.2","26.1.1"],
		"1.21":["1.21.11","1.21.11-pre5","1.21.10"]}}`

	// Se repite porque un fallo de orden por culpa de un map no aparece
	// siempre: aparece a veces, que es peor.
	for i := 0; i < 20; i++ {
		familias, err := leerVersiones(strings.NewReader(cuerpo))
		if err != nil {
			t.Fatal(err)
		}

		var nombres []string
		for _, f := range familias {
			nombres = append(nombres, f.Nombre)
		}
		if strings.Join(nombres, ",") != "26.2,26.1,1.21" {
			t.Fatalf("intento %d: familias %v, se esperaba [26.2 26.1 1.21]", i, nombres)
		}
	}
}

// TestConstruirOpciones: la mas reciente primero, agrupada por familia.
func TestConstruirOpciones(t *testing.T) {
	opciones := construirOpciones([]familiaVersiones{
		{Nombre: "26.2", Versiones: []string{"26.2", "26.2-rc-2"}},
		{Nombre: "26.1", Versiones: []string{"26.1.2", "26.1.1"}},
		{Nombre: "1.21", Versiones: []string{"1.21.11", "1.21.10"}},
	})

	if opciones[0].Value != "LATEST" || !opciones[0].Recommended {
		t.Errorf("la primera deberia ser LATEST y recomendada, es %+v", opciones[0])
	}
	if opciones[1].Value != "26.2" || opciones[1].Note == "" {
		t.Errorf("tras LATEST deberia ir 26.2 marcada como la ultima, va %+v", opciones[1])
	}
	if opciones[1].Group != "26.2" {
		t.Errorf("26.2 deberia ir en el grupo 26.2, va en %q", opciones[1].Group)
	}
	if u := opciones[len(opciones)-1]; u.Value != "1.21.10" || u.Group != "1.21" {
		t.Errorf("la ultima deberia ser 1.21.10 del grupo 1.21, es %+v", u)
	}
}

// TestNoSeRecortaLaLista: hubo un tope de doce y estaba mal. Alguien puede
// tener un mundo de una version vieja, o unos amigos sin actualizar;
// esconderlas obliga a escribirlas a mano justo cuando mas falta hacen.
func TestNoSeRecortaLaLista(t *testing.T) {
	var familias []familiaVersiones
	esperadas := 0
	for i := 0; i < 30; i++ {
		familias = append(familias, familiaVersiones{
			Nombre:    "fam",
			Versiones: []string{"a", "b", "c"},
		})
		esperadas += 3
	}

	opciones := construirOpciones(familias)
	if len(opciones) != esperadas+1 { // +1 por LATEST
		t.Errorf("devolvio %d opciones, se esperaban %d: se esta recortando",
			len(opciones), esperadas+1)
	}
}

// TestSoloEstables: Paper SI compila candidatas y prelanzamientos, pero no son
// versiones estables y no se ofrecen, por lo mismo que no se ofrecen snapshots.
func TestSoloEstables(t *testing.T) {
	opciones := construirOpciones([]familiaVersiones{
		{Nombre: "26.2", Versiones: []string{"26.2", "26.2-rc-2"}},
		{Nombre: "1.21", Versiones: []string{"1.21.11-pre5", "1.21.10"}},
	})

	for _, o := range opciones {
		if strings.Contains(o.Value, "-") {
			t.Errorf("se colo una version no estable: %q", o.Value)
		}
	}
	if len(opciones) != 3 { // LATEST + 26.2 + 1.21.10
		t.Errorf("quedaron %d opciones, se esperaban 3: %+v", len(opciones), opciones)
	}
}

// TestSoloLatestCuandoFalla: si PaperMC no responde, el panel tiene que seguir
// dejando crear mundos. Una lista corta es mejor que un error.
func TestSoloLatestCuandoFalla(t *testing.T) {
	opciones := soloLatest()
	if len(opciones) != 1 || opciones[0].Value != "LATEST" {
		t.Errorf("se esperaba solo LATEST, hay %+v", opciones)
	}
	if opciones[0].Note == "" {
		t.Error("deberia decir que no se pudo consultar la lista")
	}
}
