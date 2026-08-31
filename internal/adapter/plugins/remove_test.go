package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRemoveQuitaElJarDeLaInstancia protege lo que pasa al subir de version.
//
// Si el .jar viejo se queda, plugins/ acaba con DOS versiones del mismo
// complemento. El servidor las carga las dos y se pelea consigo mismo, y el log
// no dice que eso es lo que ocurre.
func TestRemoveQuitaElJarDeLaInstancia(t *testing.T) {
	dir := t.TempDir()
	plugins := filepath.Join(dir, "plugins")
	if err := os.MkdirAll(plugins, 0o755); err != nil {
		t.Fatal(err)
	}

	viejo := filepath.Join(plugins, "AuthMe-6.0.0-Paper.jar")
	nuevo := filepath.Join(plugins, "AuthMe-6.1.0-Paper.jar")
	for _, f := range []string{viejo, nuevo} {
		if err := os.WriteFile(f, []byte("jar"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	s := &Store{}
	if err := s.Remove(dir, "AuthMe-6.0.0-Paper.jar"); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(viejo); !os.IsNotExist(err) {
		t.Error("el .jar viejo sigue ahi; el servidor cargaria las dos versiones")
	}
	if _, err := os.Stat(nuevo); err != nil {
		t.Error("se llevo por delante el que si tenia que quedarse")
	}
}

// TestRemoveNoSeQuejaSiNoEsta: al volver a la version de fabrica puede que ese
// archivo nunca se llegara a instalar en esta instancia.
func TestRemoveNoSeQuejaSiNoEsta(t *testing.T) {
	s := &Store{}
	if err := s.Remove(t.TempDir(), "NoExiste.jar"); err != nil {
		t.Errorf("borrar algo que no esta no es un fallo: %v", err)
	}
	if err := s.Remove(t.TempDir(), ""); err != nil {
		t.Errorf("sin archivo no hay nada que borrar: %v", err)
	}
}
