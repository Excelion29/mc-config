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

// PackRepo implementa app.PackRepo.
type PackRepo struct{ db *sql.DB }

const packColumns = `id, name, url, sha1, note, created_by, created_at`

func (r *PackRepo) List(ctx context.Context) ([]domain.Pack, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+packColumns+` FROM packs ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("listando los paquetes: %w", err)
	}
	defer rows.Close()

	var out []domain.Pack
	for rows.Next() {
		p, err := escanearPack(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PackRepo) ByID(ctx context.Context, id int64) (*domain.Pack, error) {
	p, err := escanearPack(r.db.QueryRowContext(ctx,
		`SELECT `+packColumns+` FROM packs WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrPackNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PackRepo) Create(ctx context.Context, p *domain.Pack) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO packs (name, url, sha1, note, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.Name, p.URL, p.SHA1, p.Note, p.CreatedBy, p.CreatedAt.Format(time.RFC3339))
	if err != nil {
		if esUnicoViolado(err) {
			return 0, domain.ErrPackDuplicado
		}
		return 0, fmt.Errorf("guardando el paquete: %w", err)
	}
	return res.LastInsertId()
}

func (r *PackRepo) Update(ctx context.Context, p *domain.Pack) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE packs SET name = ?, url = ?, sha1 = ?, note = ? WHERE id = ?`,
		p.Name, p.URL, p.SHA1, p.Note, p.ID)
	if err != nil {
		if esUnicoViolado(err) {
			return domain.ErrPackDuplicado
		}
		return fmt.Errorf("actualizando el paquete: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrPackNotFound
	}
	return nil
}

// Delete borra el paquete y, en cascada, sus asignaciones.
//
// La cascada la hace la base: las claves foraneas van activadas en la conexion.
// Sin eso quedarian filas en world_packs apuntando a un paquete que ya no
// existe, y el mundo diria que lleva un paquete sin nombre ni enlace.
func (r *PackRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM packs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("borrando el paquete: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrPackNotFound
	}
	return nil
}

// DeMundo devuelve los paquetes de un mundo, con el activo el primero.
func (r *PackRepo) DeMundo(ctx context.Context, worldID int64) ([]domain.PackAsignado, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, p.name, p.url, p.sha1, p.note, p.created_by, p.created_at, wp.activo
		   FROM world_packs wp
		   JOIN packs p ON p.id = wp.pack_id
		  WHERE wp.world_id = ?
		  ORDER BY wp.activo DESC, p.name COLLATE NOCASE`, worldID)
	if err != nil {
		return nil, fmt.Errorf("leyendo los paquetes del mundo: %w", err)
	}
	defer rows.Close()

	var out []domain.PackAsignado
	for rows.Next() {
		var (
			a      domain.PackAsignado
			creado string
			activo int
		)
		if err := rows.Scan(&a.ID, &a.Name, &a.URL, &a.SHA1, &a.Note,
			&a.CreatedBy, &creado, &activo); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, creado)
		a.Activo = activo == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

// Asignar fija de una vez los paquetes de un mundo y cual esta activo.
//
// Se reescribe entero en una transaccion en vez de ir anadiendo y quitando: es
// lo que manda el formulario -la lista completa- y asi no hay estados
// intermedios en los que el mundo tenga dos activos o ninguno.
func (r *PackRepo) Asignar(ctx context.Context, worldID int64, ids []int64, activo int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("asignando paquetes: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM world_packs WHERE world_id = ?`, worldID); err != nil {
		return fmt.Errorf("limpiando los paquetes del mundo: %w", err)
	}

	for _, id := range ids {
		esActivo := 0
		if id == activo {
			esActivo = 1
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO world_packs (world_id, pack_id, activo) VALUES (?, ?, ?)`,
			worldID, id, esActivo); err != nil {
			return fmt.Errorf("asignando el paquete %d: %w", id, err)
		}
	}
	return tx.Commit()
}

// ActivoDe devuelve lo que el servidor necesita para aplicar el paquete.
//
// Devuelve una referencia vacia si el mundo no tiene ninguno activo, que es lo
// normal: la mayoria de los mundos no llevan paquete.
func (r *PackRepo) ActivoDe(ctx context.Context, worldID int64) (domain.PackRef, error) {
	var ref domain.PackRef
	var requerido int

	err := r.db.QueryRowContext(ctx,
		`SELECT p.url, p.sha1, w.pack_required
		   FROM world_packs wp
		   JOIN packs p  ON p.id = wp.pack_id
		   JOIN worlds w ON w.id = wp.world_id
		  WHERE wp.world_id = ? AND wp.activo = 1`, worldID).
		Scan(&ref.URL, &ref.SHA1, &requerido)

	if errors.Is(err, sql.ErrNoRows) {
		return domain.PackRef{}, nil
	}
	if err != nil {
		return domain.PackRef{}, fmt.Errorf("leyendo el paquete activo: %w", err)
	}
	ref.Required = requerido == 1
	return ref, nil
}

// SetRequired decide si el paquete activo se exige o se ofrece.
func (r *PackRepo) SetRequired(ctx context.Context, worldID int64, requerido bool) error {
	v := 0
	if requerido {
		v = 1
	}
	if _, err := r.db.ExecContext(ctx,
		`UPDATE worlds SET pack_required = ? WHERE id = ?`, v, worldID); err != nil {
		return fmt.Errorf("guardando si el paquete es obligatorio: %w", err)
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

type filaEscaneable interface {
	Scan(dest ...any) error
}

func escanearPack(row filaEscaneable) (domain.Pack, error) {
	var (
		p      domain.Pack
		creado string
	)
	if err := row.Scan(&p.ID, &p.Name, &p.URL, &p.SHA1, &p.Note,
		&p.CreatedBy, &creado); err != nil {
		return domain.Pack{}, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, creado)
	return p, nil
}
