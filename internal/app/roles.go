package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Casos de uso de RBAC con jerarquia.
//
// Dos reglas atraviesan todo este archivo:
//
//  1. Solo se gestiona lo que esta ESTRICTAMENTE por debajo. Un administrador
//     no edita el rol Administrador ni toca a otro administrador.
//  2. El rol Superusuario es intocable y unico. Es la garantia de que nunca
//     queda un panel bloqueado porque dos iguales no pueden arreglarse entre si.

// CountSuperusers indica cuantas cuentas tienen el rol raiz. Deberia ser 0 o 1.
func (a *Auth) CountSuperusers(ctx context.Context) (int, error) {
	return a.roles.CountUsersByCode(ctx, domain.RoleCodeSuperuser)
}

func (a *Auth) ListRoles(ctx context.Context, actor *domain.User) ([]domain.Role, error) {
	if !actor.Can(domain.PermRoleManage) {
		return nil, domain.ErrForbidden
	}
	return a.roles.List(ctx)
}

// RolesForAssignment devuelve los roles que el actor puede repartir.
//
// Solo los que estan por debajo del suyo: si pudiera asignar su propio nivel,
// crearia iguales a los que luego no podria gestionar.
func (a *Auth) RolesForAssignment(ctx context.Context, actor *domain.User) ([]domain.Role, error) {
	if !actor.Can(domain.PermUserManage) {
		return nil, domain.ErrForbidden
	}

	all, err := a.roles.List(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]domain.Role, 0, len(all))
	for _, r := range all {
		if actor.Role.Outranks(&r) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (a *Auth) CreateRole(ctx context.Context, actor *domain.User, code, name string, level int, perms []domain.Permission, ip string) (*domain.Role, error) {
	if !actor.Can(domain.PermRoleManage) {
		return nil, domain.ErrForbidden
	}

	code = strings.ToLower(strings.TrimSpace(code))
	name = strings.TrimSpace(name)
	if code == "" {
		return nil, domain.ErrEmptyRoleCode
	}
	if name == "" {
		name = code
	}

	// Un rol nuevo nace siempre por debajo de quien lo crea. Sin esto se
	// podrian fabricar iguales o superiores y saltarse la jerarquia entera.
	if level <= 0 || level >= actor.RoleLevel() {
		return nil, domain.ErrRoleLevelTooHigh
	}

	set, err := validatePermissions(perms)
	if err != nil {
		return nil, err
	}
	// Tampoco se pueden repartir permisos que uno mismo no tiene.
	if err := a.onlyOwnPermissions(actor, set); err != nil {
		return nil, err
	}

	role := &domain.Role{
		Code:        code,
		Name:        name,
		System:      false,
		Level:       level,
		Permissions: set,
		CreatedAt:   a.clock(),
	}

	id, err := a.roles.Create(ctx, role)
	if err != nil {
		return nil, err
	}
	role.ID = id

	a.audit.Record(ctx, actor, actor.Email, domain.ActionRoleCreated,
		fmt.Sprintf("%s (%d permisos)", role.Name, len(set)), ip)
	return role, nil
}

// SetRolePermissions reemplaza el conjunto de permisos de un rol.
func (a *Auth) SetRolePermissions(ctx context.Context, actor *domain.User, roleID int64, perms []domain.Permission, ip string) error {
	if !actor.Can(domain.PermRoleManage) {
		return domain.ErrForbidden
	}

	role, err := a.roles.ByID(ctx, roleID)
	if err != nil {
		return err
	}
	if role.IsSuperuser() {
		return domain.ErrSuperuserLocked
	}
	// Editar tu propio rol o uno de igual nivel seria escalar privilegios.
	if !actor.Role.Outranks(role) {
		return domain.ErrRoleAboveYou
	}

	set, err := validatePermissions(perms)
	if err != nil {
		return err
	}
	if err := a.onlyOwnPermissions(actor, set); err != nil {
		return err
	}

	if err := a.roles.SetPermissions(ctx, roleID, set); err != nil {
		return err
	}

	a.audit.Record(ctx, actor, actor.Email, domain.ActionRoleUpdated,
		fmt.Sprintf("%s -> %d permisos", role.Name, len(set)), ip)
	return nil
}

func (a *Auth) DeleteRole(ctx context.Context, actor *domain.User, roleID int64, ip string) error {
	if !actor.Can(domain.PermRoleManage) {
		return domain.ErrForbidden
	}

	role, err := a.roles.ByID(ctx, roleID)
	if err != nil {
		return err
	}
	if role.System {
		return domain.ErrSystemRole
	}
	if !actor.Role.Outranks(role) {
		return domain.ErrRoleAboveYou
	}

	// Se comprueba antes de borrar para poder dar un mensaje util. La clave
	// foranea RESTRICT es la red de seguridad, no la primera linea.
	n, err := a.roles.CountUsers(ctx, roleID)
	if err != nil {
		return err
	}
	if n > 0 {
		return domain.ErrRoleInUse
	}

	if err := a.roles.Delete(ctx, roleID); err != nil {
		return err
	}

	a.audit.Record(ctx, actor, actor.Email, domain.ActionRoleDeleted, role.Name, ip)
	return nil
}

// EnsureRootRole garantiza que existe el superusuario con todos los permisos.
//
// Es lo UNICO que el arranque impone. Los demas roles se crean desde el panel:
// son datos, y crearlos aqui obligaria a borrarlos despues a mano.
//
// Los permisos del rol raiz se recalculan siempre: sin eso, cada permiso nuevo
// que se anadiera al catalogo dejaria al propietario del panel sin acceso a la
// funcion recien creada.
func (a *Auth) EnsureRootRole(ctx context.Context) (*domain.Role, error) {
	full := domain.NewPermissionSet(domain.AllPermissions()...)

	existing, err := a.roles.ByCode(ctx, domain.RootRole.Code)
	switch {
	case err == nil:
		if len(full) != len(existing.Permissions) {
			if err := a.roles.SetPermissions(ctx, existing.ID, full); err != nil {
				return nil, err
			}
			existing.Permissions = full
			a.log.Info("permisos del superusuario actualizados con el catalogo",
				"permisos", len(full))
		}
		return existing, nil

	case errors.Is(err, domain.ErrRoleNotFound):
		role := &domain.Role{
			Code:        domain.RootRole.Code,
			Name:        domain.RootRole.Name,
			System:      true,
			Level:       domain.RootRole.Level,
			Permissions: full,
			CreatedAt:   a.clock(),
		}
		id, err := a.roles.Create(ctx, role)
		if err != nil {
			return nil, fmt.Errorf("creando el rol superusuario: %w", err)
		}
		role.ID = id
		a.log.Info("rol superusuario creado", "permisos", len(full))
		return role, nil

	default:
		return nil, err
	}
}

// onlyOwnPermissions impide repartir permisos que uno mismo no tiene.
// Sin esta comprobacion, quien puede gestionar roles podria fabricarse uno con
// todo y escalar hasta el nivel del superusuario.
func (a *Auth) onlyOwnPermissions(actor *domain.User, set domain.PermissionSet) error {
	for _, p := range set.List() {
		if !actor.Can(p) {
			return fmt.Errorf("%w: %q", domain.ErrForbidden, p)
		}
	}
	return nil
}

// validatePermissions descarta cualquier codigo que no este en el catalogo.
// Los permisos llegan de un formulario, asi que son entrada no confiable.
func validatePermissions(perms []domain.Permission) (domain.PermissionSet, error) {
	set := domain.PermissionSet{}
	for _, p := range perms {
		if _, ok := domain.PermissionByCode(p); !ok {
			return nil, fmt.Errorf("%w: %q", domain.ErrUnknownPermiso, p)
		}
		set[p] = struct{}{}
	}
	return set, nil
}
