package app

import (
	"context"
	"io"
	"os"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Puertos de la biblioteca de mapas.

// MapRepo persiste la biblioteca de mapas (F2).
type MapRepo interface {
	Create(ctx context.Context, m *domain.Map) (int64, error)
	ByID(ctx context.Context, id int64) (*domain.Map, error)
	BySHA(ctx context.Context, sha string) (*domain.Map, error)
	List(ctx context.Context) ([]domain.Map, error)
	ListPage(ctx context.Context, limit, offset int) ([]domain.Map, int, error)
	Delete(ctx context.Context, id int64) error
}

// MapStorage guarda los archivos de los mapas fuera de la base.
type MapStorage interface {
	TempFile() (*os.File, error)
	// ArchivePath da la ruta del archivo guardado, que F3 necesita para
	// extraer el mundo al crear una instancia.
	ArchivePath(sha string) string
	SaveArchive(sha, tempPath string) error
	SaveIcon(sha string, data []byte) error
	ReadIcon(sha string) ([]byte, error)
	Delete(sha string) error
	FreeSpace() (uint64, error)
	// DiskUsage da libre y capacidad, para poder hablar en porcentaje. El
	// espacio libre a secas no dice nada: 2 GB sobran en un disco de 20 y
	// son una urgencia en uno de 500.
	DiskUsage() (libre, total uint64, err error)
}

// MapInspector deduce que es un archivo subido.
//
// Es un puerto porque todo lo que sabe de ZIP y NBT vive en un adaptador: los
// casos de uso no deberian conocer el formato de level.dat.
type MapInspector interface {
	Inspect(r io.ReaderAt, size int64) (*domain.MapInspection, error)
}
