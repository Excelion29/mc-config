package app

import (
	"context"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Puertos de autenticacion, usuarios y roles (RBAC).

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
