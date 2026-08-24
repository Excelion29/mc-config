package mojang

import "testing"

// TestConGuiones fija el detalle que no da error pero rompe todo: Mojang
// devuelve el UUID sin guiones y los archivos de Java lo esperan con ellos.
//
// El valor sale de una respuesta REAL: el servidor de la VPS escribio
// "UUID of player Areku29 is 1c9bedc5-1bf5-43cb-be42-931dace7be8f" al
// rechazar la conexion, y esa es la forma que hay que producir.
func TestConGuiones(t *testing.T) {
	got := ConGuiones("1c9bedc51bf543cbbe42931dace7be8f")
	quiero := "1c9bedc5-1bf5-43cb-be42-931dace7be8f"
	if got != quiero {
		t.Errorf("ConGuiones = %q, se esperaba %q", got, quiero)
	}

	// Si ya viene con guiones, o no mide lo que debe, se deja como esta: mas
	// vale pasarlo tal cual que estropear algo que quiza ya era correcto.
	for _, v := range []string{quiero, "", "corto"} {
		if got := ConGuiones(v); got != v {
			t.Errorf("ConGuiones(%q) = %q, deberia dejarlo igual", v, got)
		}
	}
}
