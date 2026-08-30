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

// ResourceRepo implementa app.ResourceRepo.
type ResourceRepo struct{ db *sql.DB }

const resourceColumns = `id, kind, url, name, auto_name, sha1, directo, probado,
	note, created_by, created_at`

func (r *ResourceRepo) List(ctx context.Context, kind domain.ResourceKind) ([]domain.Resource, error) {
	consulta := `SELECT ` + resourceColumns + ` FROM resources`
	var args []any
	if kind != "" {
		consulta += ` WHERE kind = ?`
		args = append(args, string(kind))
	}
	// Se ordena por la etiqueta visible, no por el nombre: los que no tienen
	// nombre se ensenan por su enlace, y ordenar por una columna vacia los
	// amontonaria todos al principio sin ningun criterio.
	consulta += ` ORDER BY COALESCE(NULLIF(name, ''), NULLIF(auto_name, ''), url) COLLATE NOCASE`

	rows, err := r.db.QueryContext(ctx, consulta, args...)
	if err != nil {
		return nil, fmt.Errorf("listando los recursos: %w", err)
	}
	defer rows.Close()

	var out []domain.Resource
	for rows.Next() {
		res, err := escanearResource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, rows.Err()
}

func (r *ResourceRepo) ByID(ctx context.Context, id int64) (*domain.Resource, error) {
	return r.uno(r.db.QueryRowContext(ctx,
		`SELECT `+resourceColumns+` FROM resources WHERE id = ?`, id))
}

// PorURL busca por enlace, que es lo que identifica al recurso de verdad.
//
// Lo usa el camino corto -pegar un enlace en la ficha de un mundo- para
// reutilizar el recurso si ya estaba en la biblioteca en vez de duplicarlo.
func (r *ResourceRepo) PorURL(ctx context.Context, url string) (*domain.Resource, error) {
	return r.uno(r.db.QueryRowContext(ctx,
		`SELECT `+resourceColumns+` FROM resources WHERE url = ?`, url))
}

func (r *ResourceRepo) uno(row *sql.Row) (*domain.Resource, error) {
	res, err := escanearResource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrResourceNotFound
	}
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *ResourceRepo) Create(ctx context.Context, res *domain.Resource) (int64, error) {
	out, err := r.db.ExecContext(ctx,
		`INSERT INTO resources (kind, url, name, auto_name, sha1, directo, probado,
		                        note, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(res.Kind), res.URL, res.Name, res.AutoName, res.SHA1,
		booleano(res.Directo), booleano(res.Probado), res.Note,
		res.CreatedBy, res.CreatedAt.Format(time.RFC3339))
	if err != nil {
		if esUnicoViolado(err) {
			return 0, domain.ErrResourceDuplicado
		}
		return 0, fmt.Errorf("guardando el recurso: %w", err)
	}
	return out.LastInsertId()
}

func (r *ResourceRepo) Update(ctx context.Context, res *domain.Resource) error {
	out, err := r.db.ExecContext(ctx,
		`UPDATE resources SET url = ?, name = ?, auto_name = ?, sha1 = ?,
		        directo = ?, probado = ?, note = ?
		  WHERE id = ?`,
		res.URL, res.Name, res.AutoName, res.SHA1,
		booleano(res.Directo), booleano(res.Probado), res.Note, res.ID)
	if err != nil {
		if esUnicoViolado(err) {
			return domain.ErrResourceDuplicado
		}
		return fmt.Errorf("actualizando el recurso: %w", err)
	}
	if n, _ := out.RowsAffected(); n == 0 {
		return domain.ErrResourceNotFound
	}
	return nil
}

// Delete borra el recurso y, en cascada, sus asignaciones.
//
// La cascada la hace la base: las claves foraneas van activadas en la conexion.
// Sin eso quedarian filas en world_resources apuntando a un recurso que ya no
// existe, y el mundo diria que lleva algo sin enlace ni nombre.
func (r *ResourceRepo) Delete(ctx context.Context, id int64) error {
	out, err := r.db.ExecContext(ctx, `DELETE FROM resources WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("borrando el recurso: %w", err)
	}
	if n, _ := out.RowsAffected(); n == 0 {
		return domain.ErrResourceNotFound
	}
	return nil
}

// DeMundo devuelve los recursos de un mundo, con el principal el primero.
func (r *ResourceRepo) DeMundo(ctx context.Context, worldID int64) ([]domain.ResourceAsignado, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.id, e.kind, e.url, e.name, e.auto_name, e.sha1, e.directo,
		        e.probado, e.note, e.created_by, e.created_at, we.principal
		   FROM world_resources we
		   JOIN resources e ON e.id = we.resource_id
		  WHERE we.world_id = ?
		  ORDER BY we.principal DESC,
		           COALESCE(NULLIF(e.name, ''), NULLIF(e.auto_name, ''), e.url) COLLATE NOCASE`,
		worldID)
	if err != nil {
		return nil, fmt.Errorf("leyendo los recursos del mundo: %w", err)
	}
	defer rows.Close()

	var out []domain.ResourceAsignado
	for rows.Next() {
		var (
			a                domain.ResourceAsignado
			kind             string
			creado           string
			directo, probado int
			principal        int
		)
		if err := rows.Scan(&a.ID, &kind, &a.URL, &a.Name, &a.AutoName, &a.SHA1,
			&directo, &probado, &a.Note, &a.CreatedBy, &creado, &principal); err != nil {
			return nil, err
		}
		a.Kind = domain.ResourceKind(kind)
		a.Directo, a.Probado = directo == 1, probado == 1
		a.CreatedAt, _ = time.Parse(time.RFC3339, creado)
		a.Principal = principal == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

// Asignar fija de una vez los recursos de un mundo y cual es el principal.
//
// Se reescribe entero en una transaccion en vez de ir anadiendo y quitando: es
// lo que manda el formulario -la lista completa- y asi no hay ningun momento en
// el que el mundo tenga dos principales, que es justo lo que el indice unico
// rechazaria.
func (r *ResourceRepo) Asignar(ctx context.Context, worldID int64, ids []int64, principal int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("asignando recursos: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM world_resources WHERE world_id = ?`, worldID); err != nil {
		return fmt.Errorf("limpiando los recursos del mundo: %w", err)
	}

	for _, id := range ids {
		esPrincipal := 0
		if id == principal {
			esPrincipal = 1
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO world_resources (world_id, resource_id, principal) VALUES (?, ?, ?)`,
			worldID, id, esPrincipal); err != nil {
			return fmt.Errorf("asignando el recurso %d: %w", id, err)
		}
	}
	return tx.Commit()
}

// PrincipalDe devuelve lo que el servidor necesita para aplicar el recurso.
//
// Devuelve una referencia vacia si el mundo no tiene ninguno, que es lo normal:
// la mayoria de los mundos no llevan paquete de texturas.
func (r *ResourceRepo) PrincipalDe(ctx context.Context, worldID int64) (domain.PackRef, error) {
	var (
		ref       domain.PackRef
		requerido int
	)

	err := r.db.QueryRowContext(ctx,
		`SELECT e.url, e.sha1, w.resource_required
		   FROM world_resources we
		   JOIN resources e ON e.id = we.resource_id
		   JOIN worlds    w ON w.id = we.world_id
		  WHERE we.world_id = ? AND we.principal = 1`, worldID).
		Scan(&ref.URL, &ref.SHA1, &requerido)

	if errors.Is(err, sql.ErrNoRows) {
		return domain.PackRef{}, nil
	}
	if err != nil {
		return domain.PackRef{}, fmt.Errorf("leyendo el recurso principal: %w", err)
	}
	ref.Required = requerido == 1
	return ref, nil
}

// SetRequired decide si el recurso principal se exige o se ofrece.
func (r *ResourceRepo) SetRequired(ctx context.Context, worldID int64, requerido bool) error {
	v := 0
	if requerido {
		v = 1
	}
	if _, err := r.db.ExecContext(ctx,
		`UPDATE worlds SET resource_required = ? WHERE id = ?`, v, worldID); err != nil {
		return fmt.Errorf("guardando si el recurso es obligatorio: %w", err)
	}
	return nil
}

// esUnicoViolado reconoce el choque con el indice unico del enlace.
//
// Se mira el texto porque el driver no expone un codigo: modernc.org/sqlite
// devuelve el mensaje de SQLite tal cual. Es fragil, y por eso el error que se
// devuelve es del dominio: si algun dia deja de reconocerlo, lo que se rompe es
// el mensaje bonito, no el guardado.
func esUnicoViolado(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func booleano(b bool) int {
	if b {
		return 1
	}
	return 0
}

type filaEscaneable interface {
	Scan(dest ...any) error
}

func escanearResource(row filaEscaneable) (domain.Resource, error) {
	var (
		res              domain.Resource
		kind             string
		creado           string
		directo, probado int
	)
	if err := row.Scan(&res.ID, &kind, &res.URL, &res.Name, &res.AutoName,
		&res.SHA1, &directo, &probado, &res.Note, &res.CreatedBy, &creado); err != nil {
		return domain.Resource{}, err
	}
	res.Kind = domain.ResourceKind(kind)
	res.Directo, res.Probado = directo == 1, probado == 1
	res.CreatedAt, _ = time.Parse(time.RFC3339, creado)
	return res, nil
}
