// Package plugins descarga complementos de servidor y los instala en una
// instancia.
//
// Los archivos se guardan en una cache compartida y de ahi se copian a cada
// servidor. Bajarlos una vez por instancia seria gastar red y disco para tener
// copias identicas del mismo archivo, y en esta VPS el disco ya nos preocupa
// (M-2).
package plugins

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Excelion29/mc-config/internal/app"
)

// Store instala plugins usando una cache en disco.
type Store struct {
	cache   string
	cliente *http.Client
}

func New(cache string) (*Store, error) {
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return nil, fmt.Errorf("creando la cache de plugins %s: %w", cache, err)
	}
	return &Store{
		cache: cache,
		// Un minuto: son archivos de uno o dos megas y se bajan de GitHub.
		// Sin plazo, un servidor lento dejaria el arranque colgado sin fin.
		cliente: &http.Client{Timeout: time.Minute},
	}, nil
}

// Install deja los plugins en dataDir/plugins, descargando los que falten.
func (s *Store) Install(ctx context.Context, dataDir string, lista []app.Plugin) error {
	if len(lista) == 0 {
		return nil
	}

	destino := filepath.Join(dataDir, "plugins")
	if err := os.MkdirAll(destino, 0o755); err != nil {
		return fmt.Errorf("creando %s: %w", destino, err)
	}

	for _, p := range lista {
		enCache := filepath.Join(s.cache, p.File)
		if err := s.asegurar(ctx, p, enCache); err != nil {
			return err
		}
		if err := copiar(enCache, filepath.Join(destino, p.File)); err != nil {
			return fmt.Errorf("instalando %s: %w", p.Name, err)
		}
	}
	return nil
}

// Installed dice cuales de la lista ya estan en la instancia.
func (s *Store) Installed(dataDir string, lista []app.Plugin) []app.Plugin {
	var out []app.Plugin
	for _, p := range lista {
		if _, err := os.Stat(filepath.Join(dataDir, "plugins", p.File)); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// asegurar descarga el plugin a la cache si no esta ya.
//
// No se comprueba si hay una version mas nueva: una actualizacion silenciosa de
// un plugin de autenticacion es lo ultimo que queremos. Si un dia hay que
// subirlo, se cambia la URL y el nombre del archivo en el catalogo, y entonces
// el cambio esta escrito y es visible en un diff.
func (s *Store) asegurar(ctx context.Context, p app.Plugin, ruta string) error {
	if info, err := os.Stat(ruta); err == nil && info.Size() > 0 {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
	if err != nil {
		return err
	}
	resp, err := s.cliente.Do(req)
	if err != nil {
		return fmt.Errorf("descargando %s: %w", p.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("descargando %s: el servidor respondio %d", p.Name, resp.StatusCode)
	}

	// A un temporal y luego se renombra: si la descarga se corta a medias, no
	// queda un .jar truncado en la cache que luego se copie a todas las
	// instancias y falle en cada arranque sin decir por que.
	tmp := ruta + ".parcial"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, resp.Body)
	cerrar := f.Close()
	if err == nil {
		err = cerrar
	}
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("guardando %s: %w", p.Name, err)
	}

	return os.Rename(tmp, ruta)
}

func copiar(origen, destino string) error {
	in, err := os.Open(origen)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(destino)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if cerrar := out.Close(); err == nil {
		err = cerrar
	}
	return err
}
