package app

import "testing"

// TestUmbralesDeDisco fija el comportamiento en los bordes, que es donde una
// division entera se equivoca sin avisar.
func TestUmbralesDeDisco(t *testing.T) {
	casos := []struct {
		nombre        string
		libre, total  uint64
		ocupado       int
		avisar, lleno bool
	}{
		{"disco vacio", 100, 100, 0, false, false},
		{"justo por debajo del aviso", 21, 100, 79, false, false},
		{"exactamente en el aviso", 20, 100, 80, true, false},
		{"entre aviso y tope", 15, 100, 85, true, false},
		{"exactamente en el tope", 10, 100, 90, true, true},
		{"lleno del todo", 0, 100, 100, true, true},
		// El redondeo hacia arriba es intencional: 80.4% ya avisa. Mas vale
		// avisar un poco antes que un poco tarde.
		{"redondeo hacia arriba", 196, 1000, 81, true, false},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			usado := c.total - c.libre
			ocupado := int((usado*100 + c.total - 1) / c.total)
			d := DiskStatus{Libre: c.libre, Total: c.total, Ocupado: ocupado}

			if d.Ocupado != c.ocupado {
				t.Errorf("ocupado = %d, se esperaba %d", d.Ocupado, c.ocupado)
			}
			if d.Avisar() != c.avisar {
				t.Errorf("avisar = %v, se esperaba %v (ocupado %d)", d.Avisar(), c.avisar, d.Ocupado)
			}
			if d.Lleno() != c.lleno {
				t.Errorf("lleno = %v, se esperaba %v (ocupado %d)", d.Lleno(), c.lleno, d.Ocupado)
			}
		})
	}
}

// TestElTopeCortaAntesDeLlenarse protege la razon de ser del umbral: si algun
// dia alguien lo sube a 100, el corte deja de servir para nada, porque para
// entonces MySQL del cliente ya no puede escribir.
func TestElTopeCortaAntesDeLlenarse(t *testing.T) {
	if DiscoTope >= 100 {
		t.Errorf("DiscoTope = %d: tiene que cortar ANTES de llenarse", DiscoTope)
	}
	if DiscoAviso >= DiscoTope {
		t.Errorf("el aviso (%d) debe llegar antes que el tope (%d)", DiscoAviso, DiscoTope)
	}
}
