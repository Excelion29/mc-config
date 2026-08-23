// Package storage guarda los archivos de los mapas en disco.
//
// Implementa el puerto app.WorldStorage. El almacenamiento es por hash del
// contenido: dos subidas del mismo archivo comparten carpeta y el duplicado se
// detecta solo.
package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type FileStore struct {
	root string
}

func New(root string) (*FileStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("creando el almacen de mapas en %s: %w", root, err)
	}
	return &FileStore{root: root}, nil
}

func (s *FileStore) dir(sha string) string {
	// Se parte el hash en dos niveles para no acabar con miles de carpetas
	// hermanas, que en algunos sistemas de archivos degrada el listado.
	return filepath.Join(s.root, sha[:2], sha)
}

func (s *FileStore) ArchivePath(sha string) string {
	return filepath.Join(s.dir(sha), "original")
}

func (s *FileStore) IconPath(sha string) string {
	return filepath.Join(s.dir(sha), "icon")
}

// SaveArchive mueve el archivo temporal a su sitio definitivo.
//
// Se usa rename cuando es posible: es atomico y no duplica el archivo en disco.
// Si el temporal esta en otro volumen, rename falla y se copia.
func (s *FileStore) SaveArchive(sha, tempPath string) error {
	if err := os.MkdirAll(s.dir(sha), 0o755); err != nil {
		return fmt.Errorf("creando carpeta del mapa: %w", err)
	}

	dst := s.ArchivePath(sha)
	if err := os.Rename(tempPath, dst); err == nil {
		return nil
	}
	return copyFile(tempPath, dst)
}

func (s *FileStore) SaveIcon(sha string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if err := os.MkdirAll(s.dir(sha), 0o755); err != nil {
		return fmt.Errorf("creando carpeta del mapa: %w", err)
	}
	if err := os.WriteFile(s.IconPath(sha), data, 0o644); err != nil {
		return fmt.Errorf("guardando la miniatura: %w", err)
	}
	return nil
}

func (s *FileStore) ReadIcon(sha string) ([]byte, error) {
	return os.ReadFile(s.IconPath(sha))
}

func (s *FileStore) Delete(sha string) error {
	if err := os.RemoveAll(s.dir(sha)); err != nil {
		return fmt.Errorf("borrando los archivos del mapa: %w", err)
	}
	return nil
}

// TempFile crea un temporal en el mismo volumen que el almacen, para que
// SaveArchive pueda usar rename en lugar de copiar.
func (s *FileStore) TempFile() (*os.File, error) {
	tmp := filepath.Join(s.root, "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return nil, fmt.Errorf("creando carpeta temporal: %w", err)
	}
	f, err := os.CreateTemp(tmp, "upload-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("creando archivo temporal: %w", err)
	}
	return f, nil
}

// FreeSpace devuelve los bytes libres en el volumen del almacen.
func (s *FileStore) FreeSpace() (uint64, error) {
	libre, _, err := diskUsage(s.root)
	return libre, err
}

// DiskUsage devuelve el espacio libre y la capacidad de donde viven los mapas.
func (s *FileStore) DiskUsage() (libre, total uint64, err error) {
	return diskUsage(s.root)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("abriendo el temporal: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creando el destino: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copiando el archivo: %w", err)
	}
	if err := out.Sync(); err != nil {
		return fmt.Errorf("sincronizando el archivo: %w", err)
	}
	os.Remove(src)
	return nil
}
