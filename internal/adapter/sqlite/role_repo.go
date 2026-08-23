package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// RoleRepo implementa app.RoleRepo.
type RoleRepo struct{ db *sql.DB }

func (r *RoleRepo) Create(ctx context.Context, role *domain.Role) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("abriendo transaccion: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO roles (code, name, is_system, level, created_at) VALUES (?, ?, ?, ?, ?)`,
		role.Code, role.Name, boolToInt(role.System), role.Level, role.CreatedAt.Format(time.RFC3339))
	if err != nil {
		if isUniqueViolation(err) {
			return 0, domain.ErrDuplicateRole
		}
		return 0, fmt.Errorf("insertando rol: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("leyendo id del rol: %w", err)
	}

	if err := replacePermissions(ctx, tx, id, role.Permissions); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("confirmando la creacion del rol: %w", err)
	}
	return id, nil
}

func (r *RoleRepo) ByID(ctx context.Context, id int64) (*domain.Role, error) {
	return r.scanOne(ctx, `SELECT id, code, name, is_system, level, created_at FROM roles WHERE id = ?`, id)
}

func (r *RoleRepo) ByCode(ctx context.Context, code string) (*domain.Role, error) {
	return r.scanOne(ctx, `SELECT id, code, name, is_system, level, created_at FROM roles WHERE code = ?`, code)
}

func (r *RoleRepo) List(ctx context.Context) ([]domain.Role, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, code, name, is_system, level, created_at
		   FROM roles ORDER BY level DESC, created_at ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("listando roles: %w", err)
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, *role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recorriendo roles: %w", err)
	}

	// Los permisos se cargan aparte para no depender de GROUP_CONCAT, que no es
	// portable entre motores (D-14).
	for i := range roles {
		perms, err := r.permissionsOf(ctx, roles[i].ID)
		if err != nil {
			return nil, err
		}
		roles[i].Permissions = perms
	}
	return roles, nil
}

// SetPermissions reemplaza el conjunto completo de permisos del rol.
func (r *RoleRepo) SetPermissions(ctx context.Context, roleID int64, perms domain.PermissionSet) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("abriendo transaccion: %w", err)
	}
	defer tx.Rollback()

	if err := replacePermissions(ctx, tx, roleID, perms); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("confirmando los permisos: %w", err)
	}
	return nil
}

func (r *RoleRepo) Rename(ctx context.Context, roleID int64, name string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE roles SET name = ? WHERE id = ?`, name, roleID)
	if err != nil {
		return fmt.Errorf("renombrando rol: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrRoleNotFound
	}
	return nil
}

func (r *RoleRepo) Delete(ctx context.Context, roleID int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM roles WHERE id = ?`, roleID)
	if err != nil {
		// La clave foranea es RESTRICT: si quedan usuarios, el motor se niega.
		// Es la ultima red de seguridad; el caso de uso ya lo comprueba antes.
		return fmt.Errorf("borrando rol: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrRoleNotFound
	}
	return nil
}

func (r *RoleRepo) CountUsers(ctx context.Context, roleID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role_id = ?`, roleID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("contando usuarios del rol: %w", err)
	}
	return n, nil
}

// CountUsersByCode se usa para garantizar que solo hay un superusuario.
func (r *RoleRepo) CountUsersByCode(ctx context.Context, code string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users u JOIN roles r ON r.id = u.role_id WHERE r.code = ?`,
		code).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("contando usuarios del rol %q: %w", code, err)
	}
	return n, nil
}

func (r *RoleRepo) scanOne(ctx context.Context, query string, args ...any) (*domain.Role, error) {
	row := r.db.QueryRowContext(ctx, query, args...)

	var (
		role      domain.Role
		isSystem  int
		createdAt string
	)
	err := row.Scan(&role.ID, &role.Code, &role.Name, &isSystem, &role.Level, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrRoleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("leyendo rol: %w", err)
	}

	role.System = isSystem == 1
	role.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	perms, err := r.permissionsOf(ctx, role.ID)
	if err != nil {
		return nil, err
	}
	role.Permissions = perms
	return &role, nil
}

func (r *RoleRepo) permissionsOf(ctx context.Context, roleID int64) (domain.PermissionSet, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT permission FROM role_permissions WHERE role_id = ?`, roleID)
	if err != nil {
		return nil, fmt.Errorf("leyendo permisos del rol: %w", err)
	}
	defer rows.Close()

	perms := domain.PermissionSet{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("leyendo permiso: %w", err)
		}
		perms[domain.Permission(code)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recorriendo permisos: %w", err)
	}
	return perms, nil
}

func scanRole(rows *sql.Rows) (*domain.Role, error) {
	var (
		role      domain.Role
		isSystem  int
		createdAt string
	)
	if err := rows.Scan(&role.ID, &role.Code, &role.Name, &isSystem, &role.Level, &createdAt); err != nil {
		return nil, fmt.Errorf("leyendo fila de rol: %w", err)
	}
	role.System = isSystem == 1
	role.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &role, nil
}

// replacePermissions deja el rol exactamente con el conjunto indicado.
// Borrar e insertar dentro de una transaccion evita estados a medias.
func replacePermissions(ctx context.Context, tx *sql.Tx, roleID int64, perms domain.PermissionSet) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id = ?`, roleID); err != nil {
		return fmt.Errorf("limpiando permisos del rol: %w", err)
	}
	for _, p := range perms.List() {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO role_permissions (role_id, permission) VALUES (?, ?)`,
			roleID, string(p)); err != nil {
			return fmt.Errorf("asignando permiso %q: %w", p, err)
		}
	}
	return nil
}
