package java

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Excelion29/mc-config/internal/domain"
)

func propiedadesDe(t *testing.T, inst *domain.Instance) string {
	t.Helper()

	dir := t.TempDir()
	f := New(nil)
	if err := f.WriteConfig(inst, dir, nil); err != nil {
		t.Fatal(err)
	}
	datos, err := os.ReadFile(filepath.Join(dir, "server.properties"))
	if err != nil {
		t.Fatal(err)
	}
	return string(datos)
}

func instanciaConPack(pack domain.PackRef) *domain.Instance {
	return &domain.Instance{
		Name: "servidor", Slug: "servidor", LevelName: "mundo",
		Edition: domain.EditionJava, Pack: pack,
		Rules: domain.Rules{Gamemode: "survival", Difficulty: "normal", MaxPlayers: 12},
	}
}

// TestElEnlaceDelPaqueteVaEscapado protege un detalle que no da error.
//
// Un .properties usa los dos puntos y el igual como separadores, y una URL
// lleva los dos: "https://" y cualquier "?a=b". Sin escapar, el servidor lee la
// clave cortada por donde no es.
//
// Y no se queja: arranca igual y los jugadores simplemente no ven las texturas.
func TestElEnlaceDelPaqueteVaEscapado(t *testing.T) {
	props := propiedadesDe(t, instanciaConPack(domain.PackRef{
		URL:  "https://ejemplo.com/texturas.zip?v=2",
		SHA1: "0123456789abcdef0123456789abcdef01234567",
	}))

	if !strings.Contains(props, `resource-pack=https\://ejemplo.com/texturas.zip?v\=2`) {
		t.Errorf("el enlace no salio escapado:\n%s", props)
	}
	if !strings.Contains(props, "resource-pack-sha1=0123456789abcdef0123456789abcdef01234567") {
		t.Error("falta el hash, y sin el se descarga en cada conexion")
	}
}

// TestSinPaqueteNoSeEscribenLasClaves comprueba que no queden vacias.
//
// Dejar "resource-pack=" puesto no es lo mismo que no ponerlo, y averiguar en
// que versiones da igual es trabajo que no hace falta hacer.
func TestSinPaqueteNoSeEscribenLasClaves(t *testing.T) {
	props := propiedadesDe(t, instanciaConPack(domain.PackRef{}))

	for _, clave := range []string{"resource-pack", "resource-pack-sha1", "require-resource-pack"} {
		if strings.Contains(props, clave+"=") {
			t.Errorf("sin paquete no deberia aparecer %q:\n%s", clave, props)
		}
	}
}

// TestSinHashNoSeEscribeUnoFalso: un hash que no cuadra hace que el cliente
// RECHACE el paquete, asi que es peor que no tenerlo.
func TestSinHashNoSeEscribeUnoFalso(t *testing.T) {
	props := propiedadesDe(t, instanciaConPack(domain.PackRef{
		URL: "https://ejemplo.com/texturas.zip",
	}))

	if strings.Contains(props, "resource-pack-sha1=") {
		t.Errorf("sin hash no deberia escribirse la clave:\n%s", props)
	}
	if !strings.Contains(props, "require-resource-pack=false") {
		t.Error("por defecto el paquete se ofrece, no se exige")
	}
}
