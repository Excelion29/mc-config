package resources

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestInspeccionarMiraLoQueResponde comprueba las dos ramas contra servidores
// de verdad, no contra cabeceras inventadas.
//
// El caso que importa es el primero: una URL SIN extension que devuelve el
// archivo. La regla vieja -mirar si acaba en .zip- lo marcaba "a mano" y dejaba
// sin aplicar un paquete que funcionaba perfectamente.
func TestInspeccionarMiraLoQueResponde(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/descargar":
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", `attachment; filename="texturas.zip"`)
			fmt.Fprint(w, "esto hace de paquete")
		case "/pagina":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<html><head><title>Texturas Medievales &amp; Co
			| MiSitio</title></head><body>bajar</body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := New()

	t.Run("una URL sin extension que da el archivo", func(t *testing.T) {
		info, err := p.Inspeccionar(context.Background(), srv.URL+"/descargar")
		if err != nil {
			t.Fatal(err)
		}
		if !info.Probado || !info.Directo {
			t.Errorf("deberia reconocerse como archivo: %+v", info)
		}
		if info.SHA1 == "" {
			t.Error("de un archivo hay que sacar la huella, o el cliente lo baja en cada conexion")
		}
		if info.Titulo != "" {
			t.Errorf("un archivo no tiene titulo que leer, dio %q", info.Titulo)
		}
	})

	t.Run("una pagina de descarga", func(t *testing.T) {
		info, err := p.Inspeccionar(context.Background(), srv.URL+"/pagina")
		if err != nil {
			t.Fatal(err)
		}
		if !info.Probado {
			t.Error("se abrio, asi que cuenta como comprobado")
		}
		if info.Directo {
			t.Error("una pagina NO puede aplicarse sola: el juego recibiria HTML")
		}
		if info.SHA1 != "" {
			t.Error("no hay archivo que hashear")
		}
		// Las entidades deshechas y las lineas juntadas: un <title> partido en
		// dos es de lo mas normal.
		if info.Titulo != "Texturas Medievales & Co | MiSitio" {
			t.Errorf("titulo mal leido: %q", info.Titulo)
		}
	})

	t.Run("un enlace roto no cuenta como comprobado", func(t *testing.T) {
		if _, err := p.Inspeccionar(context.Background(), srv.URL+"/no-existe"); err == nil {
			t.Error("un 404 tiene que dar error, no pasar por pagina vacia")
		}
	})
}
