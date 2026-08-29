package java

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Excelion29/mc-config/internal/app"
)

// TestEscapePropertyValue fija el detalle que rompe en silencio: en un
// .properties de Java los dos puntos van escapados. Se comprobo en el archivo
// que genera un servidor real (H-J-11).
func TestEscapePropertyValue(t *testing.T) {
	casos := []struct{ entrada, quiero string }{
		{"minecraft:normal", `minecraft\:normal`},
		{"normal", "normal"},
		{"A Minecraft Server", "A Minecraft Server"},
		{`C:\ruta`, `C\:\\ruta`},
		{"clave=valor", `clave\=valor`},
		// Un salto de linea partiria el archivo y la clave siguiente se
		// leeria como parte del valor.
		{"dos\nlineas", `dos\nlineas`},
		// Los acentos y las eñes pasan tal cual: el motd los admite.
		{"Mundo de Peña", "Mundo de Peña"},
	}
	for _, c := range casos {
		if got := escapePropertyValue(c.entrada); got != c.quiero {
			t.Errorf("escape(%q) = %q, se esperaba %q", c.entrada, got, c.quiero)
		}
	}
}

func TestWriteWhitelist(t *testing.T) {
	dir := t.TempDir()

	err := WriteWhitelist(dir, []app.PlayerRef{
		{ID: "1c9bedc5-1bf5-43cb-be42-931dace7be8f", Name: "Areku29"},
		// Sin UUID: el servidor no sabria a quien se refiere. Se descarta en
		// vez de escribir una entrada que no significa nada.
		{ID: "", Name: "SinResolver"},
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "whitelist.json"))
	if err != nil {
		t.Fatal(err)
	}
	quiero := `[{"uuid":"1c9bedc5-1bf5-43cb-be42-931dace7be8f","name":"Areku29"}]`
	if string(data) != quiero {
		t.Errorf("whitelist.json =\n  %s\nse esperaba\n  %s", data, quiero)
	}
}

func TestWriteOps(t *testing.T) {
	dir := t.TempDir()
	if err := WriteOps(dir, []app.PlayerRef{
		{ID: "1c9bedc5-1bf5-43cb-be42-931dace7be8f", Name: "Areku29"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "ops.json"))
	if err != nil {
		t.Fatal(err)
	}
	quiero := `[{"uuid":"1c9bedc5-1bf5-43cb-be42-931dace7be8f","name":"Areku29",` +
		`"level":4,"bypassesPlayerLimit":false}]`
	if string(data) != quiero {
		t.Errorf("ops.json =\n  %s\nse esperaba\n  %s", data, quiero)
	}
}

// TestListasVacias: se escribe el archivo igualmente, con una lista vacia.
func TestListasVacias(t *testing.T) {
	dir := t.TempDir()
	for _, caso := range []struct {
		nombre string
		f      func(string, []app.PlayerRef) error
		arch   string
	}{
		{"whitelist", WriteWhitelist, "whitelist.json"},
		{"ops", WriteOps, "ops.json"},
	} {
		if err := caso.f(dir, nil); err != nil {
			t.Fatalf("%s: %v", caso.nombre, err)
		}
		data, err := os.ReadFile(filepath.Join(dir, caso.arch))
		if err != nil {
			t.Fatalf("%s no se escribio: %v", caso.arch, err)
		}
		if string(data) != "[]" {
			t.Errorf("%s = %s, se esperaba []", caso.arch, data)
		}
	}
}
