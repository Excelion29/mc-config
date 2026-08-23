package app

import (
	"context"
	"fmt"

	"github.com/Excelion29/mc-config/internal/domain"
)

// Gestion de usuarios del panel.
//
// El permiso se comprueba aqui y no solo en las rutas, para que no dependa de
// que alguien recuerde poner el middleware. Si se anade una ruta nueva y se
// olvida el middleware, la regla sigue aplicandose.

func (a *Auth) ListUsers(ctx context.Context, actor *domain.User) ([]domain.User, error) {
	if !actor.Can(domain.PermUserManage) {
		return nil, domain.ErrForbidden
	}
	return a.users.List(ctx)
}

// AddUser crea un usuario del panel y lo deja registrado.
//
// Solo se pueden crear cuentas por debajo del propio nivel: crear un igual
// significaria crear a alguien a quien despues no se podria gestionar.
func (a *Auth) AddUser(ctx context.Context, actor *domain.User, email, password string, roleID int64, ip string) (*domain.User, error) {
	if !actor.Can(domain.PermUserManage) {
		return nil, domain.ErrForbidden
	}

	role, err := a.roles.ByID(ctx, roleID)
	if err != nil {
		return nil, err
	}
	if role.IsSuperuser() {
		return nil, domain.ErrOnlyOneSuperuser
	}
	if !actor.Role.Outranks(role) {
		return nil, domain.ErrRoleAboveYou
	}

	u, err := a.CreateUser(ctx, email, password, roleID)
	if err != nil {
		return nil, err
	}

	a.audit.Record(ctx, actor, actor.Email, domain.ActionUserCreated,
		fmt.Sprintf("%s como %s", u.Email, u.RoleName()), ip)
	return u, nil
}

// SetUserActive activa o desactiva una cuenta.
//
// Desactivar cierra las sesiones abiertas de esa persona en la siguiente
// peticion (lo resuelve UserFromSession), no cuando caduque su cookie.
func (a *Auth) SetUserActive(ctx context.Context, actor *domain.User, id int64, active bool, ip string) error {
	if !actor.Can(domain.PermUserManage) {
		return domain.ErrForbidden
	}
	// Sin esta regla, un admin distraido puede dejar el panel sin nadie que
	// pueda entrar a arreglarlo.
	if actor.ID == id && !active {
		return domain.ErrSelfDisable
	}

	target, err := a.users.ByID(ctx, id)
	if err != nil {
		return err
	}
	if err := canManage(actor, target); err != nil {
		return err
	}
	if err := a.users.SetActive(ctx, id, active); err != nil {
		return err
	}

	action := domain.ActionUserDisabled
	if active {
		action = domain.ActionUserEnabled
	}
	a.audit.Record(ctx, actor, actor.Email, action, target.Email, ip)
	return nil
}

// SetUserRole cambia el rol de una cuenta.
func (a *Auth) SetUserRole(ctx context.Context, actor *domain.User, id, roleID int64, ip string) error {
	if !actor.Can(domain.PermUserManage) {
		return domain.ErrForbidden
	}

	target, err := a.users.ByID(ctx, id)
	if err != nil {
		return err
	}
	role, err := a.roles.ByID(ctx, roleID)
	if err != nil {
		return err
	}
	if err := canManage(actor, target); err != nil {
		return err
	}
	// Tampoco se puede ascender a alguien hasta el propio nivel: quedaria fuera
	// del alcance de quien acaba de ascenderlo.
	if role.IsSuperuser() {
		return domain.ErrOnlyOneSuperuser
	}
	if !actor.Role.Outranks(role) {
		return domain.ErrRoleAboveYou
	}

	if err := a.users.SetRole(ctx, id, roleID); err != nil {
		return err
	}

	a.audit.Record(ctx, actor, actor.Email, domain.ActionUserRoleChanged,
		fmt.Sprintf("%s: %s -> %s", target.Email, target.RoleName(), role.Name), ip)
	return nil
}

// canManage traduce la regla de jerarquia del dominio a un error explicativo.
func canManage(actor, target *domain.User) error {
	if actor.CanManage(target) {
		return nil
	}
	switch {
	case target.Role.IsSuperuser():
		return domain.ErrSuperuserLocked
	case actor.ID == target.ID:
		return domain.ErrSelfDisable
	default:
		return domain.ErrSamePeer
	}
}
