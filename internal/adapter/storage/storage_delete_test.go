package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBorrarSinHashNoTocaNada cubre los mundos creados desde una semilla.
//
// Esos no subieron ningun archivo, asi que no tienen hash (D-16). El almacen
// reparte los archivos por los dos primeros caracteres del hash, y con la
// cadena vacia eso reventaba: "slice bounds out of range [:2] with length 0".
//
// El panic no era lo peor. Lo peor habria sido "arreglarlo" devolviendo la raiz
// cuando no hay hash: Delete hace RemoveAll, asi que borrar un mundo creado se
// habria llevado por delante la biblioteca ENTERA, con todos los mapas
// importados dentro. Por eso esta prueba comprueba las dos cosas.
func TestBorrarSinHashNoTocaNada(t *testing.T) {
	raiz := t.TempDir()

	store, err := New(raiz)
	if err != nil {
		t.Fatal(err)
	}

	// Un mapa importado cualquiera, para tener algo que perder.
	sha := "abcdef0123456789"
	dir := filepath.Join(raiz, sha[:2], sha)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "original"), []byte("mapa"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := store.Delete(""); err != nil {
		t.Fatalf("borrar sin hash no deberia fallar: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "original")); err != nil {
		t.Fatal("borrar un mundo sin archivos se llevo por delante los de otro mapa")
	}
	if _, err := os.Stat(raiz); err != nil {
		t.Fatal("la biblioteca entera desaparecio")
	}
}

// TestBorrarConHashSiBorra comprueba que la salvaguarda no rompio lo normal.
func TestBorrarConHashSiBorra(t *testing.T) {
	raiz := t.TempDir()

	store, err := New(raiz)
	if err != nil {
		t.Fatal(err)
	}

	sha := "abcdef0123456789"
	dir := filepath.Join(raiz, sha[:2], sha)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := store.Delete(sha); err != nil {
		t.Fatalf("borrar con hash deberia funcionar: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("los archivos del mapa siguen ahi")
	}
}
