package app

import (
	"context"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Puertos de sesion, registro de acciones y criptografia.
//
// Van juntos porque los tres sirven a lo mismo: saber quien entra, dejar
// constancia de lo que hace y proteger sus credenciales.

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
