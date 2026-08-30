package resources

import (
	"net/http"
	"testing"
)

// TestQueRespuestaEsUnArchivo fija como se distingue el paquete de una pagina.
//
// Antes se adivinaba por la extension de la URL, y fallaba en los dos sentidos:
// hay CDN que sirven el archivo desde "/pack?id=123" -sin extension- y paginas
// de descarga que acaban en ".zip". Se equivocaba justo donde importa, porque
// marcar como automatico algo que devuelve HTML rompe la conexion de todos.
func TestQueRespuestaEsUnArchivo(t *testing.T) {
	casos := []struct {
		nombre   string
		cabecera map[string]string
		archivo  bool
	}{
		{"un zip declarado", map[string]string{"Content-Type": "application/zip"}, true},
		{"zip con juego de caracteres pegado", map[string]string{"Content-Type": "application/zip; charset=binary"}, true},
		{"la variante de Windows", map[string]string{"Content-Type": "application/x-zip-compressed"}, true},
		{
			nombre:   "bytes sin tipo concreto, que es lo que Minecraft espera",
			cabecera: map[string]string{"Content-Type": "application/octet-stream"},
			archivo:  true,
		},
		{
			nombre: "el servidor dice que es para guardar, aunque el tipo sea raro",
			cabecera: map[string]string{
				"Content-Type":        "text/html",
				"Content-Disposition": `attachment; filename="texturas.zip"`,
			},
			archivo: true,
		},
		{"una pagina de descarga", map[string]string{"Content-Type": "text/html; charset=utf-8"}, false},
		{"una imagen", map[string]string{"Content-Type": "image/png"}, false},
		{"sin decir nada", map[string]string{}, false},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			h := http.Header{}
			for k, v := range c.cabecera {
				h.Set(k, v)
			}
			if got := EsDescarga(h); got != c.archivo {
				t.Errorf("EsDescarga(%v) = %v, se esperaba %v", c.cabecera, got, c.archivo)
			}
		})
	}
}
