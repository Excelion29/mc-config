package mcworld

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Extract descomprime el mundo dentro de destDir.
//
// Es la operacion mas peligrosa del panel: escribe archivos venidos de internet
// en el disco de una maquina que tambien aloja la produccion de un cliente
// (D-09). Por eso cada entrada se valida DOS veces: al inspeccionar el archivo
// al subirlo, y otra vez aqui. Un mapa pudo entrar a la biblioteca con una
// version anterior del validador.
func (Inspector) Extract(archivePath, destDir string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return domain.ErrNotAnArchive
	}
	defer zr.Close()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creando %s: %w", destDir, err)
	}

	// Se resuelve la ruta real del destino para poder comprobar despues que
	// nada acaba fuera, incluso si hay enlaces simbolicos por medio.
	root, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("resolviendo el destino: %w", err)
	}

	// Si el autor comprimio la carpeta en vez de su contenido, todo cuelga de
	// un prefijo comun que hay que quitar: el servidor espera level.dat en la
	// raiz del mundo.
	prefix := commonPrefix(zr.File)

	var total int64
	for _, f := range zr.File {
		if !safePath(f.Name) {
			return fmt.Errorf("%w: %q", domain.ErrUnsafePath, f.Name)
		}

		rel := strings.TrimPrefix(toSlash(f.Name), prefix)
		if rel == "" {
			continue
		}

		target := filepath.Join(root, filepath.FromSlash(rel))

		// Segunda barrera: se comprueba el resultado, no solo la entrada.
		// filepath.Join limpia la ruta, asi que esto atrapa cualquier caso raro
		// que safePath no previera.
		if !strings.HasPrefix(target, root+string(os.PathSeparator)) && target != root {
			return fmt.Errorf("%w: %q escaparia a %s", domain.ErrUnsafePath, f.Name, target)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("creando carpeta %s: %w", target, err)
			}
			continue
		}

		total += int64(f.UncompressedSize64)
		if total > maxUncompressed {
			return domain.ErrZipBomb
		}

		if err := extractFile(f, target); err != nil {
			return err
		}
	}

	return nil
}

func extractFile(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("creando carpeta de %s: %w", target, err)
	}

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("abriendo %s dentro del archivo: %w", f.Name, err)
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("creando %s: %w", target, err)
	}
	defer out.Close()

	// LimitReader por si el tamano declarado en la cabecera miente.
	if _, err := io.Copy(out, io.LimitReader(rc, maxUncompressed)); err != nil {
		return fmt.Errorf("escribiendo %s: %w", target, err)
	}
	return nil
}

// commonPrefix detecta si todo el contenido cuelga de una sola carpeta.
// Devuelve el prefijo a recortar, con barra final, o cadena vacia.
func commonPrefix(files []*zip.File) string {
	var first string
	for _, f := range files {
		name := toSlash(f.Name)
		if name == "" {
			continue
		}

		i := strings.Index(name, "/")
		if i < 0 {
			// Hay algo en la raiz: no existe prefijo comun.
			return ""
		}

		dir := name[:i+1]
		if first == "" {
			first = dir
			continue
		}
		if dir != first {
			return ""
		}
	}
	return first
}

func toSlash(name string) string {
	return path.Clean(strings.ReplaceAll(name, `\`, "/"))
}
