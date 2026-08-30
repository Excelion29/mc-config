package java

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Excelion29/mc-config/internal/domain"
)

// configDeEjemplo imita el archivo real: con comentarios y otras claves.
const configDeEjemplo = `# FastLogin
# Documentacion: https://github.com/TuxCoding/FastLogin

# Request a premium login without forcing the player to type a command.
autoRegister: false

# Auto login the player after a successful premium check
autoLogin: true

premiumUuid: false
`

func escribir(t *testing.T, dataDir, contenido string) string {
	t.Helper()

	dir := filepath.Join(dataDir, "plugins", "FastLogin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ruta := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
	return ruta
}

// TestSeEnciendeAutoRegister protege lo que hace que un premium no tenga que
// inventarse una contrasena.
//
// FastLogin trae autoRegister en FALSE. Con eso solo entra solo quien YA tiene
// cuenta en AuthMe, y no la crea: la primera vez, a alguien con el juego
// comprado le sale la misma pantalla de "Register" que a quien no lo tiene.
// Justo lo que el modo sin conexion decia evitar.
func TestSeEnciendeAutoRegister(t *testing.T) {
	dir := t.TempDir()
	ruta := escribir(t, dir, configDeEjemplo)

	if err := (&Flavor{}).AjustarPlugins(dir, domain.AuthOffline); err != nil {
		t.Fatal(err)
	}

	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	texto := string(datos)

	if !strings.Contains(texto, "autoRegister: true") {
		t.Errorf("no se encendio autoRegister:\n%s", texto)
	}

	// Lo demas se queda EXACTAMENTE igual. Reescribir el archivo con una
	// libreria de YAML borraria los comentarios, que son la unica explicacion
	// que tiene quien abra ese archivo despues.
	if !strings.Contains(texto, "# Documentacion: https://github.com/TuxCoding/FastLogin") {
		t.Error("se perdieron los comentarios del archivo")
	}
	if !strings.Contains(texto, "autoLogin: true") || !strings.Contains(texto, "premiumUuid: false") {
		t.Errorf("se tocaron otras claves:\n%s", texto)
	}
}

// TestNoSeToraNadaEnModoNormal: sin acceso abierto no hay FastLogin en juego.
func TestNoSeTocaNadaEnModoNormal(t *testing.T) {
	dir := t.TempDir()
	ruta := escribir(t, dir, configDeEjemplo)

	if err := (&Flavor{}).AjustarPlugins(dir, domain.AuthOnline); err != nil {
		t.Fatal(err)
	}

	datos, _ := os.ReadFile(ruta)
	if string(datos) != configDeEjemplo {
		t.Error("en modo normal no habria que tocar la configuracion")
	}
}

// TestSinArchivoNoEsUnFallo cubre el primer arranque.
//
// El archivo lo crea el plugin cuando arranca, asi que la primera vez no existe.
// Se ajusta en el arranque siguiente, porque esto se ejecuta cada vez.
func TestSinArchivoNoEsUnFallo(t *testing.T) {
	if err := (&Flavor{}).AjustarPlugins(t.TempDir(), domain.AuthOffline); err != nil {
		t.Errorf("la primera vez no hay archivo, y eso no es un error: %v", err)
	}
}

// TestNoSeInventaLaClaveSiNoEsta: si FastLogin la renombra en una version nueva,
// anadirla no arreglaria nada y ensuciaria el archivo.
func TestNoSeInventaLaClaveSiNoEsta(t *testing.T) {
	original := "# otra version\nalgunaOtraClave: false\n"
	dir := t.TempDir()
	ruta := escribir(t, dir, original)

	if err := (&Flavor{}).AjustarPlugins(dir, domain.AuthOffline); err != nil {
		t.Fatal(err)
	}

	datos, _ := os.ReadFile(ruta)
	if string(datos) != original {
		t.Errorf("no deberia anadirse una clave que el plugin no reconoce:\n%s", datos)
	}
}

// TestNoSeConfundeConUnaClaveDeOtraSeccion: una con sangria pertenece a otro
// bloque y no es la que se busca.
func TestNoSeConfundeConUnaClaveDeOtraSeccion(t *testing.T) {
	original := "otraSeccion:\n  autoRegister: false\n"
	dir := t.TempDir()
	ruta := escribir(t, dir, original)

	if err := (&Flavor{}).AjustarPlugins(dir, domain.AuthOffline); err != nil {
		t.Fatal(err)
	}

	datos, _ := os.ReadFile(ruta)
	if string(datos) != original {
		t.Errorf("se cambio una clave que estaba dentro de otra seccion:\n%s", datos)
	}
}
