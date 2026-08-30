// Package resources mira que hay al otro lado del enlace de un recurso.
package resources

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Excelion29/mc-config/internal/app"
)

// maxArchivo es lo mas que se descarga para hashear.
//
// El enlace es de otro: si apunta a algo enorme -o a un flujo sin fin- el panel
// se quedaria descargando por una tarea que es de comodidad, no imprescindible.
const maxArchivo = 250 << 20 // 250 MB

// maxPagina es lo que se lee de una pagina para sacarle el titulo.
//
// El titulo va en la cabecera del HTML, asi que con los primeros kilobytes
// sobra. Leer la pagina entera para quedarse con una linea seria descargar un
// sitio de descargas completo, anuncios incluidos.
const maxPagina = 64 << 10 // 64 KB

var (
	reOGTitle = regexp.MustCompile(`(?is)<meta[^>]+property\s*=\s*["']og:title["'][^>]+content\s*=\s*["']([^"']+)["']`)
	reTitle   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

// Probe averigua si un enlace da el archivo, y de paso su huella o su titulo.
type Probe struct{ cliente *http.Client }

func New() *Probe {
	return &Probe{cliente: &http.Client{Timeout: 3 * time.Minute}}
}

// Inspeccionar abre el enlace una vez y decide que es.
//
// Lo decide por lo que RESPONDE, no por como se llama la URL. Adivinar por la
// extension se equivocaba en los dos sentidos: hay CDN que sirven el paquete
// desde "/pack?id=123", sin extension y perfectamente validos, y paginas de
// descarga que acaban en ".zip" sin serlo.
//
// Segun lo que sea, hace una cosa u otra:
//
//   - ARCHIVO: se descarga entero para calcular el SHA-1 y se tiran los bytes.
//     Ese hash evita que el cliente se lo baje otra vez en cada conexion, y
//     nadie lo tiene a mano: no viene en la pagina ni lo publica el autor.
//   - PAGINA: no hay hash que calcular -no es el paquete- pero si un titulo que
//     leer, y con eso el enlace deja de tener que bautizarse a mano.
//
// En ningun caso se guarda nada en disco: el panel no aloja recursos (M-2).
func (p *Probe) Inspeccionar(ctx context.Context, url string) (app.Inspeccion, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return app.Inspeccion{}, err
	}

	resp, err := p.cliente.Do(req)
	if err != nil {
		return app.Inspeccion{}, fmt.Errorf("abriendo el enlace: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return app.Inspeccion{}, fmt.Errorf("el enlace respondio %d", resp.StatusCode)
	}

	// A partir de aqui SI se pudo mirar, pase lo que pase con el contenido.
	info := app.Inspeccion{Probado: true}

	if EsDescarga(resp.Header) {
		info.Directo = true
		info.SHA1, err = hashear(resp.Body)
		return info, err
	}

	info.Titulo, err = titular(resp.Body)
	return info, err
}

// EsDescarga decide si la respuesta es el archivo y no una pagina.
//
// Se mira primero Content-Disposition: un servidor que dice "attachment" esta
// mandando algo para guardar, y eso no admite discusion.
//
// Despues el tipo de contenido. "application/octet-stream" entra a proposito
// aunque sea generico: significa "bytes que no se pintan", que es justo lo que
// Minecraft espera recibir.
//
// Ante la duda se dice que NO. Marcar como automatico algo que resulta ser una
// pagina rompe la conexion de todos; marcarlo a mano solo obliga a instalarlo
// aparte.
func EsDescarga(h http.Header) bool {
	if disp, _, err := mime.ParseMediaType(h.Get("Content-Disposition")); err == nil {
		if strings.EqualFold(disp, "attachment") {
			return true
		}
	}

	tipo, _, err := mime.ParseMediaType(h.Get("Content-Type"))
	if err != nil {
		return false
	}

	switch strings.ToLower(tipo) {
	case "application/zip", "application/x-zip-compressed", "application/x-zip",
		"application/octet-stream", "binary/octet-stream":
		return true
	}
	return false
}

// hashear suma los bytes segun pasan, sin memoria ni disco de por medio.
func hashear(cuerpo io.Reader) (string, error) {
	suma := sha1.New()
	if _, err := io.Copy(suma, io.LimitReader(cuerpo, maxArchivo)); err != nil {
		return "", fmt.Errorf("leyendo el archivo: %w", err)
	}
	return hex.EncodeToString(suma.Sum(nil)), nil
}

// titular saca el nombre que la pagina se da a si misma.
//
// Manda og:title sobre <title>: es el que los sitios ponen para compartir en
// redes, asi que suele venir limpio, mientras que <title> arrastra el nombre del
// sitio detras.
//
// Con expresiones regulares y no con un analizador de HTML entero. Es lo que se
// puede defender: aqui no se INTERPRETA la pagina ni se sigue nada de lo que
// diga, solo se copia un texto para ensenarlo como etiqueta. Si se equivoca, lo
// peor que pasa es que el nombre quede feo y haya que escribirlo a mano.
func titular(cuerpo io.Reader) (string, error) {
	datos, err := io.ReadAll(io.LimitReader(cuerpo, maxPagina))
	if err != nil {
		return "", fmt.Errorf("leyendo la pagina: %w", err)
	}

	for _, re := range []*regexp.Regexp{reOGTitle, reTitle} {
		if m := re.FindSubmatch(datos); m != nil {
			return limpiarTitulo(string(m[1])), nil
		}
	}
	return "", nil
}

// limpiarTitulo deja el titulo presentable.
//
// Se deshacen las entidades -"&amp;" tiene que volver a ser "&"- y se juntan
// los espacios, porque un <title> partido en varias lineas es de lo mas normal.
// Se recorta porque hay sitios que meten la descripcion entera ahi dentro.
func limpiarTitulo(t string) string {
	t = strings.Join(strings.Fields(html.UnescapeString(t)), " ")
	if len(t) > 120 {
		t = strings.TrimSpace(t[:120]) + "..."
	}
	return t
}
