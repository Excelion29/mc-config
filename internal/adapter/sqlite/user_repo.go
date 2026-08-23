package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Excelion29/mc-config/internal/domain"
)

// UserRepo implementa app.UserRepo.
//
// Al leer un usuario se carga tambien su rol con los permisos: Can() tiene que
// poder responder sin volver a la base.
type UserRepo struct {
	db    *sql.DB
	roles *RoleRepo
}

// Se selecciona TODO lo que necesita domain.Role, nivel incluido: si falta el
// nivel, Outranks compara ceros y la jerarquia bloquea a todo el mundo.
const userColumns = `u.id, u.email, u.password_hash, u.role_id, u.active, u.created_at,
	r.id, r.code, r.name, r.is_system, r.level, r.created_at`

const userFrom = `FROM users u JOIN roles r ON r.id = u.role_id`

func (r *UserRepo) Create(ctx context.Context, u *domain.User) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO users (email, password_hash, role_id, active, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		u.Email, u.PasswordHash, u.RoleID, boolToInt(u.Active),
		u.CreatedAt.Format(time.RFC3339))
	if err != nil {
		// El error de unicidad se traduce a un error del dominio para que las
		// capas de arriba no tengan que leer mensajes del motor (D-14).
		if isUniqueViolation(err) {
			return 0, domain.ErrDuplicateEmail
		}
		return 0, fmt.Errorf("insertando usuario: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("leyendo id del usuario: %w", err)
	}
	return id, nil
}

func (r *UserRepo) ByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.scanOne(ctx, `SELECT `+userColumns+` `+userFrom+` WHERE u.email = ?`, email)
}

func (r *UserRepo) ByID(ctx context.Context, id int64) (*domain.User, error) {
	return r.scanOne(ctx, `SELECT `+userColumns+` `+userFrom+` WHERE u.id = ?`, id)
}

func (r *UserRepo) List(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+userColumns+` `+userFrom+` ORDER BY u.created_at ASC, u.id ASC`)
	if err != nil {
		return nil, fmt.Errorf("listando usuarios: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		u, err := scanUserRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recorriendo usuarios: %w", err)
	}

	// Los permisos se resuelven despues, cacheando por rol: una lista de veinte
	// usuarios con tres roles hace tres consultas, no veinte.
	cache := map[int64]domain.PermissionSet{}
	for i := range users {
		perms, ok := cache[users[i].RoleID]
		if !ok {
			perms, err = r.roles.permissionsOf(ctx, users[i].RoleID)
			if err != nil {
				return nil, err
			}
			cache[users[i].RoleID] = perms
		}
		users[i].Role.Permissions = perms
	}
	return users, nil
}

func (r *UserRepo) SetActive(ctx context.Context, id int64, active bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET active = ? WHERE id = ?`, boolToInt(active), id)
	if err != nil {
		return fmt.Errorf("cambiando estado del usuario: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("comprobando el cambio de estado: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepo) SetRole(ctx context.Context, id, roleID int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE users SET role_id = ? WHERE id = ?`, roleID, id)
	if err != nil {
		return fmt.Errorf("cambiando el rol del usuario: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepo) Count(ctx context.Context) (int, error) {
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("contando usuarios: %w", err)
	}
	return n, nil
}

func (r *UserRepo) scanOne(ctx context.Context, query string, args ...any) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, query, args...)

	u, err := scanUserRow(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	perms, err := r.roles.permissionsOf(ctx, u.RoleID)
	if err != nil {
		return nil, err
	}
	u.Role.Permissions = perms
	return u, nil
}

// scanUserRow comparte el escaneo entre QueryRow y Query, que exponen Scan con
// la misma firma pero en tipos distintos.
func scanUserRow(scan func(...any) error) (*domain.User, error) {
	var (
		u             domain.User
		role          domain.Role
		active        int
		isSystem      int
		userCreated   string
		roleCreatedAt string
	)
	err := scan(&u.ID, &u.Email, &u.PasswordHash, &u.RoleID, &active, &userCreated,
		&role.ID, &role.Code, &role.Name, &isSystem, &role.Level, &roleCreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("leyendo usuario: %w", err)
	}

	u.Active = active == 1
	u.CreatedAt, _ = time.Parse(time.RFC3339, userCreated)

	role.System = isSystem == 1
	role.CreatedAt, _ = time.Parse(time.RFC3339, roleCreatedAt)
	u.Role = &role

	return &u, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isUniqueViolation(err error) bool {
	return strings.Contains(strings.ToUpper(err.Error()), "UNIQUE")
}
