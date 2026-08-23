// Package app contiene los casos de uso y los puertos que necesitan.
//
// Los puertos se declaran aqui, donde se consumen, no en el paquete que los
// implementa. Es lo idiomatico en Go y evita que app dependa de sus adaptadores:
// la flecha de dependencia siempre apunta hacia adentro.
//
//	adapter/web  ->  app  ->  domain
//	adapter/sqlite   (implementa los puertos de app)
package app

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// UserRepo persiste usuarios del panel.
//
// Las lecturas devuelven el usuario con su rol y permisos ya cargados: Can()
// debe poder responder sin volver a la base.
type UserRepo interface {
	Create(ctx context.Context, u *domain.User) (int64, error)
	ByEmail(ctx context.Context, email string) (*domain.User, error)
	ByID(ctx context.Context, id int64) (*domain.User, error)
	List(ctx context.Context) ([]domain.User, error)
	ListPage(ctx context.Context, limit, offset int) ([]domain.User, int, error)
	SetActive(ctx context.Context, id int64, active bool) error
	SetRole(ctx context.Context, id, roleID int64) error
	Count(ctx context.Context) (int, error)
}

// RoleRepo persiste roles y sus permisos (RBAC).
type RoleRepo interface {
	Create(ctx context.Context, r *domain.Role) (int64, error)
	ByID(ctx context.Context, id int64) (*domain.Role, error)
	ByCode(ctx context.Context, code string) (*domain.Role, error)
	List(ctx context.Context) ([]domain.Role, error)
	SetPermissions(ctx context.Context, roleID int64, perms domain.PermissionSet) error
	Rename(ctx context.Context, roleID int64, name string) error
	Delete(ctx context.Context, roleID int64) error
	CountUsers(ctx context.Context, roleID int64) (int, error)
	CountUsersByCode(ctx context.Context, code string) (int, error)
}

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
}

// MapInspector deduce que es un archivo subido.
//
// Es un puerto porque todo lo que sabe de ZIP y NBT vive en un adaptador: los
// casos de uso no deberian conocer el formato de level.dat.
type MapInspector interface {
	Inspect(r io.ReaderAt, size int64) (*domain.MapInspection, error)
}

// SessionRepo persiste sesiones de panel.
type SessionRepo interface {
	Create(ctx context.Context, s *domain.Session) error
	ByToken(ctx context.Context, token string) (*domain.Session, error)
	Delete(ctx context.Context, token string) error
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}

// AuditRepo persiste el registro de acciones (D-08).
type AuditRepo interface {
	Record(ctx context.Context, e *domain.LogEntry) error
	Latest(ctx context.Context, limit int) ([]domain.LogEntry, error)
	// Search devuelve una pagina y ademas cuantas filas cumplen el filtro.
	// El total viaja junto a las filas porque quien pinta el paginador lo
	// necesita, y pedirlo por separado abriria la puerta a que las dos
	// consultas vean estados distintos de la tabla.
	Search(ctx context.Context, text string, action domain.Action, limit, offset int) ([]domain.LogEntry, int, error)
}

// Hasher abstrae el algoritmo de contrasenas.
//
// Se declara como puerto para que cambiar de bcrypt a argon2 sea sustituir un
// adaptador, y para que los casos de uso se puedan probar con un hasher falso
// sin pagar el coste de bcrypt en cada test.
type Hasher interface {
	Hash(password string) (string, error)
	Verify(hash, password string) bool
}

// TokenGenerator produce tokens de sesion imprevisibles.
type TokenGenerator interface {
	New() (string, error)
}

// Clock permite fijar el tiempo en los tests. En produccion es time.Now.
type Clock func() time.Time
