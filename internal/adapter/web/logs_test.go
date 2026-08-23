package web

import (
	"strings"
	"testing"
)

// TestLineasNuevas es el corazon del directo: decidir que se ha escrito desde
// la ultima vuelta cuando la ventana del log se desliza.
func TestLineasNuevas(t *testing.T) {
	base := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}

	casos := []struct {
		nombre           string
		previas, actuales []string
		quiero           []string
	}{
		{
			"primera vuelta: todo es nuevo",
			nil, []string{"uno", "dos"},
			[]string{"uno", "dos"},
		},
		{
			"nada cambio",
			base, base,
			[]string{},
		},
		{
			// El caso normal: la ventana avanza, se cae lo viejo por delante
			// y aparece lo nuevo por detras. Comparar longitudes no serviria:
			// las dos listas miden igual.
			"la ventana avanza dos lineas",
			base,
			append(append([]string{}, base[2:]...), "l", "m"),
			[]string{"l", "m"},
		},
		{
			// Si el servidor escribio mas de una ventana entera entre vueltas
			// no hay solape posible. Se repite todo, que es molesto pero
			// honesto; perder lineas seria peor.
			"sin solape: se devuelve todo",
			base,
			[]string{"x", "y", "z"},
			[]string{"x", "y", "z"},
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := lineasNuevas(c.previas, c.actuales)
			if strings.Join(got, "|") != strings.Join(c.quiero, "|") {
				t.Errorf("lineasNuevas = %q, se esperaba %q", got, c.quiero)
			}
		})
	}
}

// TestLineasNuevasConRepetidas: un log de Minecraft repite lineas identicas
// constantemente. Anclarse a la ultima haria coincidir con la ocurrencia
// equivocada y reenviar historia vieja como si fuera nueva.
func TestLineasNuevasConRepetidas(t *testing.T) {
	previas := []string{
		"Running AutoCompaction...", "Running AutoCompaction...",
		"Running AutoCompaction...", "Player connected: Ana",
		"Running AutoCompaction...", "Running AutoCompaction...",
		"Running AutoCompaction...", "Running AutoCompaction...",
		"Running AutoCompaction...", "Running AutoCompaction...",
	}
	actuales := append(append([]string{}, previas...), "Player connected: Beto")

	got := lineasNuevas(previas, actuales)
	if len(got) != 1 || got[0] != "Player connected: Beto" {
		t.Errorf("lineasNuevas = %q, se esperaba solo la conexion de Beto", got)
	}
}

// TestEmitirEventoPartePorLineas: en SSE un salto de linea suelto TERMINA el
// evento. Una linea de log con salto partiria el mensaje en dos y el navegador
// veria basura.
func TestEmitirEventoPartePorLineas(t *testing.T) {
	var sb strings.Builder
	emitirEvento(&escritorFalso{&sb}, "linea", "primera\nsegunda")

	quiero := "event: linea\ndata: primera\ndata: segunda\n\n"
	if sb.String() != quiero {
		t.Errorf("evento =\n%q\nse esperaba\n%q", sb.String(), quiero)
	}
}
